package inject

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"farsiforge/pkg/core"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/scanner"
	"farsiforge/pkg/tools"
)

// ── Shared script helpers (mirror of pkg/extract) ───────────────────

// scriptSummary is the tolerant parse of the trailing "SUMMARY: {json}" line.
type scriptSummary struct {
	Reconstructed     int            `json:"reconstructed,omitempty"`
	Targets           map[string]int `json:"targets,omitempty"`
	Files             int            `json:"files,omitempty"`
	Strings           int            `json:"strings,omitempty"`
	Injected          int            `json:"injected,omitempty"`
	NotFound          int            `json:"not_found,omitempty"`
	SkippedNoTypeTree int            `json:"skipped_no_typetree,omitempty"`
	Errors            int            `json:"errors,omitempty"`
	FilesModified     int            `json:"files_modified,omitempty"`
	ModifiedFiles     []string       `json:"modified_files,omitempty"`
}

type scriptResult struct {
	Output   string
	Summary  scriptSummary
	ExitCode int
}

func toolsRegistry(reg core.ToolRegistry) *tools.Registry {
	if tr, ok := reg.(*tools.Registry); ok {
		return tr
	}
	return nil
}

// runPythonScript writes the embedded Python script into workDir and runs
// `python <script> <gameDir> <workDir> [toolsDir]` silently via
// pkg/tools.RunSilent, parsing the trailing SUMMARY line and exit code.
func runPythonScript(ctx context.Context, reg core.ToolRegistry, op, scriptName, gameDir, workDir string) (*scriptResult, error) {
	py := reg.GetPython()
	if py == "" {
		return nil, core.NewError(op, "Python is required but was not found")
	}

	scriptPath, err := tools.WriteScript(scriptName, workDir)
	if err != nil {
		return nil, core.NewError(op, "failed to write embedded script "+scriptName+": "+err.Error())
	}

	args := []string{scriptPath, gameDir, workDir}
	if tr := toolsRegistry(reg); tr != nil && tr.RootDir != "" {
		args = append(args, tr.RootDir)
	}

	out, runErr := tools.RunSilent(ctx, workDir, py, args...)
	res := &scriptResult{Output: out}
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			return res, core.NewError(op, "failed to run "+scriptName+": "+runErr.Error())
		}
	}
	res.Summary = parseSummary(out)
	return res, nil
}

func parseSummary(output string) scriptSummary {
	var s scriptSummary
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "SUMMARY: ") {
			jsonPart := strings.TrimPrefix(line, "SUMMARY: ")
			_ = json.Unmarshal([]byte(jsonPart), &s)
			break
		}
	}
	return s
}

func scriptExitError(op string, exitCode int, output string) error {
	switch exitCode {
	case 0:
		return nil
	case 2:
		return core.NewError(op, "UnityPy is not installed. Install it with: pip install UnityPy")
	case 3:
		return core.NewError(op, "game data not found at the specified path")
	case 4:
		return core.NewError(op, "script error: "+firstLines(output, 20))
	default:
		return core.NewError(op, "script failed (exit "+strconv.Itoa(exitCode)+"): "+firstLines(output, 20))
	}
}

func firstLines(s string, n int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= n {
			break
		}
	}
	return strings.Join(out, "\n")
}

func fftoolsPath(reg core.ToolRegistry) string {
	if tr := toolsRegistry(reg); tr != nil {
		return tr.FFTools
	}
	return reg.GetPath("fftools")
}

// writeCSV writes rows (each a []string) to path as a CSV file.
func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// ── Unity ───────────────────────────────────────────────────────────

type UnityInjector struct{}

func (i *UnityInjector) SupportedEngine() string { return "unity" }
func (i *UnityInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "python",
	}
}

func (i *UnityInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	res := &core.InjectionResult{}
	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		return res, core.NewError("inject", "no translated entries are ready for injection")
	}
	var rawCandidates []string
	for _, entry := range translatedEntries {
		if strings.HasPrefix(entry.Path, "raw_") && strings.EqualFold(entry.Context, "MonoBehaviour") {
			rawCandidates = append(rawCandidates, entry.ID)
		}
	}
	if len(rawCandidates) > 0 {
		return res, core.NewError("inject", fmt.Sprintf(
			"%d translated Unity entries came from a raw-byte scan and have no verified typetree field identity; refusing to stage a partial patch. Recover the matching Unity typetree and re-extract before injection",
			len(rawCandidates)))
	}
	if !reg.IsAvailable("py_UnityPy") {
		return res, core.NewError("inject", "UnityPy Python package is not installed. Install it with: pip install UnityPy")
	}
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}

	// Write processed entries to translated.json for inject.py.
	outJson := filepath.Join(workDir, "translated.json")
	toInject := make([]core.StringEntry, 0, len(translatedEntries))
	for _, e := range translatedEntries {
		e.Translation = processPersianTranslation(e.Translation, opts)
		toInject = append(toInject, e)
	}
	data, err := json.Marshal(toInject)
	if err != nil {
		return res, core.NewError("inject", "failed to marshal translated entries: "+err.Error())
	}
	if err := os.WriteFile(outJson, data, 0644); err != nil {
		return res, core.NewError("inject", "failed to write translated.json: "+err.Error())
	}

	// Run inject.py silently via pkg/tools.
	scriptRes, err := runPythonScript(ctx, reg, "inject", "inject.py", info.GameRoot, workDir)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res, nil
	}
	if scriptRes.ExitCode != 0 {
		return res, scriptExitError("inject", scriptRes.ExitCode, scriptRes.Output)
	}
	if scriptRes.Summary.Errors > 0 || scriptRes.Summary.NotFound > 0 {
		return res, fmt.Errorf("Unity injection was incomplete: injected=%d expected=%d, not_found=%d, skipped_no_typetree=%d, errors=%d",
			scriptRes.Summary.Injected, len(translatedEntries), scriptRes.Summary.NotFound,
			scriptRes.Summary.SkippedNoTypeTree, scriptRes.Summary.Errors)
	}
	if scriptRes.Summary.Injected != len(translatedEntries) || scriptRes.Summary.FilesModified == 0 {
		return res, fmt.Errorf("Unity injection produced no complete output: injected=%d expected=%d files=%d",
			scriptRes.Summary.Injected, len(translatedEntries), scriptRes.Summary.FilesModified)
	}
	res.StringCount = scriptRes.Summary.Injected
	res.ModifiedFiles, err = stageUnityOutputs(workDir, proj, scriptRes.Summary.ModifiedFiles)
	if err != nil {
		return res, err
	}
	log.Info("Unity injection completed", "reconstructed", scriptRes.Summary.Reconstructed, "output", firstLines(scriptRes.Output, 5))
	return res, nil
}

// ── Unreal Engine ───────────────────────────────────────────────────

type UnrealInjector struct{}

func (i *UnrealInjector) SupportedEngine() string { return "unreal" }
func (i *UnrealInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "fftools",
	}
}

func (i *UnrealInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	res := &core.InjectionResult{}
	fftools := fftoolsPath(reg)
	if fftools == "" {
		return res, core.NewError("inject", "fftools.exe not found (required for Unreal .locres import)")
	}

	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}

	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		return res, core.NewError("inject", "no translated entries are ready for injection")
	}
	res.StringCount = len(translatedEntries)

	// Group translated entries by their source file so we can build one CSV
	// per .locres (key,translation).
	byFile := make(map[string][]core.StringEntry)
	for _, e := range translatedEntries {
		byFile[e.File] = append(byFile[e.File], e)
	}

	for file, entries := range byFile {
		relFile, err := cleanGameRelativePath(file)
		if err != nil {
			return res, fmt.Errorf("invalid Unreal localization path %q: %w", file, err)
		}
		// Build the translations CSV (columns: key,translation).
		var rows [][]string
		rows = append(rows, []string{"key", "translation"})
		for _, e := range entries {
			rows = append(rows, []string{e.Path, processPersianTranslation(e.Translation, opts)})
		}
		csvPath, err := stagedPath(workDir, "inputs", relFile+".translations.csv")
		if err != nil {
			return res, err
		}
		if err := os.MkdirAll(filepath.Dir(csvPath), 0o755); err != nil {
			return res, fmt.Errorf("create translation input directory: %w", err)
		}
		outCsv := csvPath
		if err := writeCSV(outCsv, rows); err != nil {
			return res, fmt.Errorf("write CSV for %s: %w", file, err)
		}

		// Locate the original .locres in the game tree.
		locresPath, err := gameFilePath(info.GameRoot, relFile)
		if err != nil {
			return res, err
		}
		if !scanner.FileExists(locresPath) {
			return res, fmt.Errorf("locres not found: %s", file)
		}

		outLocres, err := stagedPath(workDir, "out", relFile)
		if err != nil {
			return res, err
		}
		if err := os.MkdirAll(filepath.Dir(outLocres), 0o755); err != nil {
			return res, fmt.Errorf("create staged localization directory: %w", err)
		}
		out, err := tools.RunSilent(ctx, workDir, fftools, "locres", "import", locresPath, outCsv, "-o", outLocres)
		if err != nil {
			return res, fmt.Errorf("locres import failed for %s: %s", file, firstLines(out, 5))
		}
		if !fileExists(outLocres) {
			return res, fmt.Errorf("locres import did not create staged output for %s", file)
		}
		res.ModifiedFiles = append(res.ModifiedFiles, filepath.ToSlash(relFile))
	}

	sort.Strings(res.ModifiedFiles)
	return res, nil
}

// ── Godot ───────────────────────────────────────────────────────────

type GodotInjector struct{}

func (i *GodotInjector) SupportedEngine() string { return "godot" }
func (i *GodotInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "fftools",
	}
}

func (i *GodotInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	return injectGodotTranslations(ctx, info, proj, reg, opts)
}

// ── Generic ─────────────────────────────────────────────────────────

type GenericInjector struct{}

func (i *GenericInjector) SupportedEngine() string { return "generic" }
func (i *GenericInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: false,
	}
}

func (i *GenericInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	res := &core.InjectionResult{}
	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		return res, core.NewError("inject", "no translated entries are ready for injection")
	}
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}
	res.StringCount = len(translatedEntries)

	// Work only on staged copies; the patch installer is responsible for
	// creating a backup when the user explicitly applies the patch.
	for _, file := range proj.ExtractedFiles {
		relFile, err := cleanGameRelativePath(file)
		if err != nil {
			return res, fmt.Errorf("invalid extracted file path %q: %w", file, err)
		}
		absPath, err := gameFilePath(info.GameRoot, relFile)
		if err != nil {
			return res, err
		}
		if !scanner.FileExists(absPath) {
			return res, fmt.Errorf("extracted source file is missing: %s", relFile)
		}
		outPath, err := stagedPath(workDir, "out", relFile)
		if err != nil {
			return res, err
		}
		changed, err := injectIntoTextFile(absPath, outPath, relFile, translatedEntries, opts)
		if err != nil {
			return res, fmt.Errorf("inject %s: %w", relFile, err)
		}
		if changed {
			res.ModifiedFiles = append(res.ModifiedFiles, filepath.ToSlash(relFile))
		}
	}
	if len(res.ModifiedFiles) == 0 {
		return res, core.NewError("inject", "no staged files contained translated keys")
	}
	sort.Strings(res.ModifiedFiles)
	return res, nil
}

// injectIntoTextFile applies translations to a generic text/ini file by
// matching entry.Path (the key before '=') and replacing the value with the
// Persian-processed translation. It reads the original and writes only to the
// caller-provided staged output path.
func injectIntoTextFile(sourcePath, outputPath, relPath string, entries []core.StringEntry, opts core.PersianOptions) (bool, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return false, err
	}
	if !utf8.Valid(data) {
		return false, fmt.Errorf("unsupported non-UTF-8 text encoding")
	}

	// Build a key→translation map for this file.
	replacements := make(map[string]string)
	for _, e := range entries {
		entryPath, err := cleanGameRelativePath(e.File)
		if err != nil || !strings.EqualFold(filepath.ToSlash(entryPath), filepath.ToSlash(relPath)) {
			continue
		}
		translation := processPersianTranslation(e.Translation, opts)
		if old, exists := replacements[e.Path]; exists && old != translation {
			return false, fmt.Errorf("multiple translations target key %q", e.Path)
		}
		replacements[e.Path] = translation
	}
	if len(replacements) == 0 {
		return false, nil
	}

	lines := strings.Split(string(data), "\n")
	changed := false
	seenKeys := make(map[string]int, len(replacements))
	for i, raw := range lines {
		content := raw
		newline := ""
		if strings.HasSuffix(content, "\r") {
			content = strings.TrimSuffix(content, "\r")
			newline = "\r"
		}
		separator := strings.IndexByte(content, '=')
		if separator < 0 {
			continue
		}
		key := strings.TrimSpace(content[:separator])
		if translation, ok := replacements[key]; ok {
			seenKeys[key]++
			if seenKeys[key] > 1 {
				return false, fmt.Errorf("translated key %q occurs more than once", key)
			}
			value := content[separator+1:]
			leading := value[:len(value)-len(strings.TrimLeft(value, " \t"))]
			trailing := value[len(strings.TrimRight(value, " \t")):]
			lines[i] = content[:separator+1] + leading + translation + trailing + newline
			changed = true
		}
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if seenKeys[key] == 0 {
			return false, fmt.Errorf("translated key %q was not found in source file", key)
		}
	}
	if !changed {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(outputPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func processPersianTranslation(text string, opts core.PersianOptions) string {
	if !(opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits || opts.ConvertPunct || opts.DropDiacritics) {
		return text
	}
	return persian.Process(text, persian.Options{
		Reshape:        opts.Reshape,
		BidiReorder:    opts.BidiReorder,
		FixYeh:         opts.FixYeh,
		PersianDigits:  opts.PersianDigits,
		ConvertPunct:   opts.ConvertPunct,
		DropDiacritics: opts.DropDiacritics,
	})
}

func cleanGameRelativePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.ContainsRune(name, 0) || path.IsAbs(name) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := path.Clean(name)
	first := strings.SplitN(clean, "/", 2)[0]
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(first, ":") {
		return "", fmt.Errorf("path escapes the game directory")
	}
	return filepath.FromSlash(clean), nil
}

func gameFilePath(gameRoot, relPath string) (string, error) {
	rel, err := cleanGameRelativePath(relPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(gameRoot, rel), nil
}

func stagedPath(workDir, area, relPath string) (string, error) {
	rel, err := cleanGameRelativePath(relPath)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(filepath.Join(workDir, area))
	if err != nil {
		return "", err
	}
	output := filepath.Join(root, rel)
	relOutput, err := filepath.Rel(root, output)
	if err != nil || relOutput == ".." || strings.HasPrefix(relOutput, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("staged output escapes its directory")
	}
	return output, nil
}

func stageUnityOutputs(workDir string, proj *core.Project, modified []string) ([]string, error) {
	if len(modified) == 0 {
		return nil, fmt.Errorf("Unity injector returned no modified asset paths")
	}
	byName := make(map[string]string, len(proj.ExtractedFiles))
	for _, file := range proj.ExtractedFiles {
		rel, err := cleanGameRelativePath(file)
		if err != nil {
			return nil, fmt.Errorf("invalid extracted Unity asset path %q: %w", file, err)
		}
		name := strings.ToLower(filepath.Base(rel))
		if previous, exists := byName[name]; exists && previous != rel {
			return nil, fmt.Errorf("ambiguous Unity asset basename %q maps to both %q and %q", name, previous, rel)
		}
		byName[name] = rel
	}
	result := make([]string, 0, len(modified))
	seen := make(map[string]bool, len(modified))
	for _, assetName := range modified {
		assetName = filepath.Base(filepath.Clean(strings.ReplaceAll(assetName, "\\", "/")))
		rel, ok := byName[strings.ToLower(assetName)]
		if !ok {
			return nil, fmt.Errorf("Unity modified asset %q is not present in the extracted project", assetName)
		}
		source, err := stagedPath(workDir, "out", assetName)
		if err != nil {
			return nil, err
		}
		target, err := stagedPath(workDir, "out", rel)
		if err != nil {
			return nil, err
		}
		if !fileExists(target) {
			return nil, fmt.Errorf("Unity staged output is missing: %s", source)
		}
		if !strings.EqualFold(filepath.Clean(source), filepath.Clean(target)) {
			if fileExists(target) {
				return nil, fmt.Errorf("multiple Unity outputs map to %s", rel)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
			if err := os.Rename(source, target); err != nil {
				return nil, fmt.Errorf("move Unity output into game-relative staging path: %w", err)
			}
		}
		if !seen[rel] {
			seen[rel] = true
			result = append(result, filepath.ToSlash(rel))
		}
	}
	sort.Strings(result)
	return result, nil
}

func fileExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}
