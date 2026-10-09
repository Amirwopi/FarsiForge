package inject

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"farsiforge/pkg/backup"
	"farsiforge/pkg/core"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/scanner"
	"farsiforge/pkg/tools"
)

// ── Shared script helpers (mirror of pkg/extract) ───────────────────

// scriptSummary is the tolerant parse of the trailing "SUMMARY: {json}" line.
type scriptSummary struct {
	Reconstructed int            `json:"reconstructed,omitempty"`
	Targets       map[string]int `json:"targets,omitempty"`
	Files         int            `json:"files,omitempty"`
	Strings       int            `json:"strings,omitempty"`
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
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}

	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		res.Warnings = append(res.Warnings, "No translated entries found")
		return res, nil
	}

	if !reg.IsAvailable("py_UnityPy") {
		return res, core.NewError("inject", "UnityPy Python package is not installed. Install it with: pip install UnityPy")
	}

	// Process Persian text for each translated entry.
	processed := make(map[string]string)
	for _, e := range translatedEntries {
		tr := e.Translation
		if opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits {
			tr = persian.Process(tr, persian.Options{
				Reshape:        opts.Reshape,
				BidiReorder:    opts.BidiReorder,
				FixYeh:         opts.FixYeh,
				PersianDigits:  opts.PersianDigits,
				ConvertPunct:   opts.ConvertPunct,
				DropDiacritics: opts.DropDiacritics,
			})
		}
		processed[e.Source] = tr
	}
	res.StringCount = len(processed)

	// Backup target files.
	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))
	for _, file := range proj.ExtractedFiles {
		if _, _, err := backupMgr.BackupFile(file); err != nil {
			res.Errors = append(res.Errors, err.Error())
		} else {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}

	// Write processed entries to translated.json for inject.py.
	outJson := filepath.Join(workDir, "translated.json")
	var toInject []core.StringEntry
	for _, e := range translatedEntries {
		e.Translation = processed[e.Source]
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
		res.Errors = append(res.Errors, scriptExitError("inject", scriptRes.ExitCode, scriptRes.Output).Error())
		return res, nil
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
		return res, nil
	}

	// Build a source→processed-translation map for Persian shaping.
	processed := make(map[string]string)
	for _, e := range translatedEntries {
		tr := e.Translation
		if opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits {
			tr = persian.Process(tr, persian.Options{
				Reshape:        opts.Reshape,
				BidiReorder:    opts.BidiReorder,
				FixYeh:         opts.FixYeh,
				PersianDigits:  opts.PersianDigits,
				ConvertPunct:   opts.ConvertPunct,
				DropDiacritics: opts.DropDiacritics,
			})
		}
		processed[e.Source] = tr
	}
	res.StringCount = len(processed)

	// Backup target files.
	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))
	for _, file := range proj.ExtractedFiles {
		if _, _, err := backupMgr.BackupFile(file); err == nil {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}

	// Group translated entries by their source file so we can build one CSV
	// per .locres (key,translation).
	byFile := make(map[string][]core.StringEntry)
	for _, e := range translatedEntries {
		byFile[e.File] = append(byFile[e.File], e)
	}

	for file, entries := range byFile {
		// Build the translations CSV (columns: key,translation).
		var rows [][]string
		rows = append(rows, []string{"key", "translation"})
		for _, e := range entries {
			rows = append(rows, []string{e.Path, processed[e.Source]})
		}
		outCsv := filepath.Join(workDir, filepath.Base(file)+".translations.csv")
		if err := writeCSV(outCsv, rows); err != nil {
			res.Errors = append(res.Errors, "failed to write CSV for "+file+": "+err.Error())
			continue
		}

		// Locate the original .locres in the game tree.
		locresPath := file
		if !filepath.IsAbs(locresPath) {
			locresPath = filepath.Join(info.GameRoot, file)
		}
		if !scanner.FileExists(locresPath) {
			res.Errors = append(res.Errors, "locres not found: "+file)
			continue
		}

		outLocres := filepath.Join(workDir, filepath.Base(file))
		out, err := tools.RunSilent(ctx, workDir, fftools, "locres", "import", locresPath, outCsv, "-o", outLocres)
		if err != nil {
			res.Errors = append(res.Errors, "locres import failed for "+file+": "+firstLines(out, 5))
			continue
		}
	}

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
	res := &core.InjectionResult{}
	fftools := fftoolsPath(reg)
	if fftools == "" {
		return res, core.NewError("inject", "fftools.exe not found (required for Godot translation import)")
	}

	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}

	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		return res, nil
	}

	// Process Persian text.
	processed := make(map[string]string)
	for _, e := range translatedEntries {
		tr := e.Translation
		if opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits {
			tr = persian.Process(tr, persian.Options{
				Reshape:        opts.Reshape,
				BidiReorder:    opts.BidiReorder,
				FixYeh:         opts.FixYeh,
				PersianDigits:  opts.PersianDigits,
				ConvertPunct:   opts.ConvertPunct,
				DropDiacritics: opts.DropDiacritics,
			})
		}
		processed[e.Source] = tr
	}
	res.StringCount = len(processed)

	// Backup target files.
	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))
	for _, file := range proj.ExtractedFiles {
		if _, _, err := backupMgr.BackupFile(file); err == nil {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}

	// Group translated entries by source file (one CSV per .translation).
	byFile := make(map[string][]core.StringEntry)
	for _, e := range translatedEntries {
		byFile[e.File] = append(byFile[e.File], e)
	}

	translationToolAvailable := true
	for file, entries := range byFile {
		var rows [][]string
		rows = append(rows, []string{"key", "translation"})
		for _, e := range entries {
			rows = append(rows, []string{e.Path, processed[e.Source]})
		}
		outCsv := filepath.Join(workDir, filepath.Base(file)+".translations.csv")
		if err := writeCSV(outCsv, rows); err != nil {
			res.Errors = append(res.Errors, "failed to write CSV for "+file+": "+err.Error())
			continue
		}

		// The patched .translation is written next to the original.
		translationPath := file
		if !filepath.IsAbs(translationPath) {
			translationPath = filepath.Join(info.GameRoot, file)
		}
		outTranslation := filepath.Join(workDir, filepath.Base(file))

		out, err := tools.RunSilent(ctx, workDir, fftools, "translation", "import", outCsv, outTranslation)
		if err != nil {
			// The translation command may not be deployed yet — degrade
			// gracefully. Report once and skip remaining files.
			if translationToolAvailable {
				res.Errors = append(res.Errors,
					"fftools translation import is not available yet — Godot .translation injection skipped")
				translationToolAvailable = false
			}
			log.Warn("fftools translation import not available yet", "file", file, "output", firstLines(out, 3))
			continue
		}
	}

	return res, nil
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

	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}
	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))

	translatedEntries := proj.FindTranslated()
	res.StringCount = len(translatedEntries)

	// Create backup for all extracted text files.
	for _, file := range proj.ExtractedFiles {
		if _, _, err := backupMgr.BackupFile(file); err == nil {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}

	// Replace text line-by-line in generic text files.
	for _, file := range proj.ExtractedFiles {
		absPath := file
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(info.GameRoot, file)
		}
		if !scanner.FileExists(absPath) {
			continue
		}
		if err := injectIntoTextFile(absPath, proj, info.GameRoot, opts); err != nil {
			res.Errors = append(res.Errors, err.Error())
		}
	}

	return res, nil
}

// injectIntoTextFile applies translations to a generic text/ini file by
// matching entry.Path (the key before '=') and replacing the value with the
// Persian-processed translation.
func injectIntoTextFile(path string, proj *core.Project, root string, opts core.PersianOptions) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	relPath, _ := filepath.Rel(root, path)

	// Build a key→translation map for this file.
	replacements := make(map[string]string)
	for _, e := range proj.FindTranslated() {
		if e.File != relPath {
			continue
		}
		tr := e.Translation
		if opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits {
			tr = persian.Process(tr, persian.Options{
				Reshape:        opts.Reshape,
				BidiReorder:    opts.BidiReorder,
				FixYeh:         opts.FixYeh,
				PersianDigits:  opts.PersianDigits,
				ConvertPunct:   opts.ConvertPunct,
				DropDiacritics: opts.DropDiacritics,
			})
		}
		replacements[e.Path] = tr
	}
	if len(replacements) == 0 {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if tr, ok := replacements[key]; ok {
			// Preserve leading whitespace of the original line.
			lead := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
			lines[i] = lead + key + "=" + tr
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
