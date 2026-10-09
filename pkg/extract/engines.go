package extract

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
	"farsiforge/pkg/textfilter"
	"farsiforge/pkg/tools"
)

// ── Shared script helpers ────────────────────────────────────────────

// scriptSummary is the tolerant parse of the trailing "SUMMARY: {json}"
// line emitted by extract.py / inject.py. Unknown fields are ignored and
// every field is optional.
type scriptSummary struct {
	Reconstructed int            `json:"reconstructed,omitempty"`
	Targets       map[string]int `json:"targets,omitempty"`
	Files         int            `json:"files,omitempty"`
	Strings       int            `json:"strings,omitempty"`
}

// scriptResult holds the outcome of running an embedded Python script.
type scriptResult struct {
	Output   string
	Summary  scriptSummary
	ExitCode int
}

// toolsRegistry extracts the concrete *tools.Registry from the
// core.ToolRegistry interface so we can access RootDir / FFTools. It returns
// nil when the registry is a mock (tests).
func toolsRegistry(reg core.ToolRegistry) *tools.Registry {
	if tr, ok := reg.(*tools.Registry); ok {
		return tr
	}
	return nil
}

// runPythonScript writes the embedded Python script (scriptName) into workDir,
// then runs `python <script> <gameDir> <workDir> [toolsDir]` silently via
// pkg/tools.RunSilent. It parses the trailing SUMMARY line and maps the
// process exit code into scriptResult.ExitCode.
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

// parseSummary scans the output backwards for the last line starting with
// "SUMMARY: " and unmarshals the JSON that follows. Missing or malformed
// SUMMARY lines yield a zero-value scriptSummary (no error).
func parseSummary(output string) scriptSummary {
	var s scriptSummary
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "SUMMARY: ") {
			jsonPart := strings.TrimPrefix(line, "SUMMARY: ")
			_ = json.Unmarshal([]byte(jsonPart), &s) // tolerant: ignore error
			break
		}
	}
	return s
}

// scriptExitError maps a Python script exit code to an actionable error.
// op is the operation name ("extract" / "inject") for the error context.
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

// firstLines returns up to n non-empty lines of s (for compact error messages).
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

// fftoolsPath returns the fftools.exe path from the registry, or "" if absent.
func fftoolsPath(reg core.ToolRegistry) string {
	if tr := toolsRegistry(reg); tr != nil {
		return tr.FFTools
	}
	return reg.GetPath("fftools")
}

// readCSV reads a CSV file and returns its rows. Returns nil if the file
// cannot be opened.
func readCSV(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate variable column counts
	rows, _ := r.ReadAll()
	return rows
}

// ── Unity ───────────────────────────────────────────────────────────

type UnityExtractor struct{}

func (e *UnityExtractor) SupportedEngine() string { return "unity" }
func (e *UnityExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "python",
	}
}

func (e *UnityExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return err
	}

	// UnityPy is a pip package; check the python-package availability flag.
	if !reg.IsAvailable("py_UnityPy") {
		return core.NewError("extract", "UnityPy Python package is not installed. Install it with: pip install UnityPy")
	}

	res, err := runPythonScript(ctx, reg, "extract", "extract.py", info.GameRoot, workDir)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return scriptExitError("extract", res.ExitCode, res.Output)
	}

	log.Debug("Unity extraction summary", "reconstructed", res.Summary.Reconstructed, "output", firstLines(res.Output, 5))

	// Read the extracted.json produced by extract.py in the work directory.
	outJson := filepath.Join(workDir, "extracted.json")
	data, err := os.ReadFile(outJson)
	if err != nil {
		return core.NewError("extract", "failed to read extracted strings: "+err.Error())
	}

	var entries []core.StringEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return core.NewError("extract", "failed to parse extracted strings: "+err.Error())
	}
	dataRoot := info.DataPath
	if dataRoot == "" {
		dataRoot = findUnityDataRoot(info.GameRoot)
	}
	if dataRoot == "" {
		return core.NewError("extract", "could not resolve the Unity data directory for game-relative asset paths")
	}
	relDataRoot, err := filepath.Rel(info.GameRoot, dataRoot)
	if err != nil || filepath.IsAbs(relDataRoot) || strings.HasPrefix(relDataRoot, ".."+string(filepath.Separator)) || relDataRoot == ".." {
		return core.NewError("extract", "Unity data directory is outside the selected game directory")
	}

	fileMap := make(map[string]bool)
	for _, e := range entries {
		if filepath.IsAbs(e.File) || strings.Contains(filepath.ToSlash(e.File), "../") || filepath.ToSlash(e.File) == ".." {
			return core.NewError("extract", "Unity asset path escaped the data directory: "+e.File)
		}
		e.File = filepath.ToSlash(filepath.Join(relDataRoot, e.File))
		proj.AddEntry(e)
		if !fileMap[e.File] {
			proj.ExtractedFiles = append(proj.ExtractedFiles, e.File)
			fileMap[e.File] = true
		}
	}

	return nil
}

func findUnityDataRoot(gameRoot string) string {
	if strings.HasSuffix(strings.ToLower(filepath.Base(filepath.Clean(gameRoot))), "_data") {
		return gameRoot
	}
	entries, err := os.ReadDir(gameRoot)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), "_data") {
			return filepath.Join(gameRoot, entry.Name())
		}
	}
	return ""
}

// ── Unreal Engine ───────────────────────────────────────────────────

type UnrealExtractor struct{}

func (e *UnrealExtractor) SupportedEngine() string { return "unreal" }
func (e *UnrealExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "fftools",
	}
}

func (e *UnrealExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return err
	}

	// 1. Find loose .locres files (search a few levels under the game root).
	locresFiles := scanner.WalkDir(info.GameRoot, 6, func(p string) bool {
		return strings.ToLower(filepath.Ext(p)) == ".locres"
	})

	// 2. Export each loose .locres to CSV via fftools.
	exportedAny := false
	for _, f := range locresFiles {
		if e.exportLocresFile(ctx, proj, reg, workDir, f, f) {
			exportedAny = true
		}
	}

	// 3. .pak archives: export .locres inside the pak and (fallback) scan
	//    .uexp exports from text-bearing directories.
	pakFiles := scanner.WalkDir(info.GameRoot, 6, func(p string) bool {
		return strings.ToLower(filepath.Ext(p)) == ".pak"
	})
	if len(pakFiles) > 0 {
		e.extractFromPak(ctx, info, proj, reg, workDir, pakFiles)
	} else if !exportedAny {
		log.Warn("No .locres files found. Ensure .pak files are unpacked first.")
	}

	return nil
}

// exportLocresFile exports a single .locres file to CSV via fftools and adds
// the entries to the project. Returns true on success.
func (e *UnrealExtractor) exportLocresFile(ctx context.Context, proj *core.Project, reg core.ToolRegistry, workDir, locresPath, displayPath string) bool {
	fftools := fftoolsPath(reg)
	if fftools == "" {
		log.Warn("fftools.exe not found (required for Unreal .locres export)")
		return false
	}

	relPath := displayPath
	if relPath == locresPath {
		relPath, _ = filepath.Rel(proj.GameRoot, locresPath)
		if relPath == "" {
			relPath = filepath.Base(locresPath)
		}
	}

	outCsv := filepath.Join(workDir, filepath.Base(locresPath)+".csv")
	out, err := tools.RunSilent(ctx, workDir, fftools, "locres", "export", locresPath, "-o", outCsv)
	if err != nil {
		log.Warn("Failed to export locres", "file", locresPath, "error", err, "output", firstLines(out, 5))
		return false
	}

	rows := readCSV(outCsv)
	count := 0
	for i, row := range rows {
		if i == 0 || len(row) < 2 {
			continue // skip header / malformed rows
		}
		key, source := row[0], row[1]
		if source == "" {
			continue
		}
		// Official locres tables also carry engine chrome that must never be
		// translated (resolution names, key labels, format-only strings:
		// "1080p24", "N/A", "F10", "Date/Time"). Skipping them here is safe:
		// locres import merges translations into the original file, so
		// untranslated keys keep their English value.
		if !textfilter.IsTranslatable(source) {
			continue
		}
		proj.AddEntry(core.StringEntry{
			Source:  source,
			File:    relPath,
			Path:    key,
			Context: "Unreal",
		})
		count++
	}
	if count > 0 {
		proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
	}
	return true
}

// repakPath returns the repak.exe path (UE4 .pak unpacker), or "" if absent.
func repakPath(reg core.ToolRegistry) string {
	if tr := toolsRegistry(reg); tr != nil {
		for _, p := range []string{
			filepath.Join(tr.RootDir, "repak", "repak.exe"),
			filepath.Join(tr.RootDir, "repak.exe"),
		} {
			if scanner.FileExists(p) {
				return p
			}
		}
	}
	return ""
}

// extractFromPak handles .pak archives:
//
//  1. .locres files inside the pak are read via `repak get` and exported
//     through fftools (the real localization text — e.g. Game.locres).
//  2. When the game hardcodes text in DataTable/Widget assets instead
//     (no .locres), .uexp files from text-bearing directories are batch-
//     unpacked with a single `repak unpack -i <dir>` per pak and scanned
//     locally. Per-file `repak get` is avoided: on big paks (tens of
//     thousands of .uexp) it takes hours.
func (e *UnrealExtractor) extractFromPak(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, workDir string, pakFiles []string) {
	repak := repakPath(reg)
	if repak == "" {
		log.Warn("repak.exe not found (required to read .pak files)")
		return
	}

	for _, pak := range pakFiles {
		listOut, err := tools.RunSilent(ctx, workDir, repak, "list", pak)
		if err != nil {
			log.Warn("Failed to list pak", "file", pak, "error", err, "output", firstLines(listOut, 5))
			continue
		}

		var locresPaths, uexpPaths []string
		for _, line := range strings.Split(listOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			switch strings.ToLower(filepath.Ext(line)) {
			case ".locres":
				locresPaths = append(locresPaths, line)
			case ".uexp":
				if isUETextPath(line) {
					uexpPaths = append(uexpPaths, line)
				}
			}
		}

		// 1) .locres inside the pak — extract + export.
		for _, lp := range locresPaths {
			data, err := tools.RunSilent(ctx, workDir, repak, "get", pak, lp)
			if err != nil {
				log.Warn("Failed to read locres from pak", "file", lp, "error", err)
				continue
			}
			local := filepath.Join(workDir, "locres_from_pak", sanitizePakName(lp))
			if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
				continue
			}
			if err := os.WriteFile(local, []byte(data), 0o644); err != nil {
				log.Warn("Failed to write locres", "file", local, "error", err)
				continue
			}
			e.exportLocresFile(ctx, proj, reg, workDir, local, lp)
		}

		// 2) .uexp fallback — batch unpack text dirs, then scan locally.
		if len(uexpPaths) == 0 {
			continue
		}
		if len(uexpPaths) > ueMaxUexp {
			log.Warn("Too many .uexp files - truncating", "total", len(uexpPaths), "max", ueMaxUexp)
			uexpPaths = uexpPaths[:ueMaxUexp]
		}
		dirs := make(map[string]bool)
		for _, p := range uexpPaths {
			idx := strings.LastIndex(p, "/")
			if idx > 0 {
				dirs[p[:idx]] = true
			}
		}
		unpackDir := filepath.Join(workDir, "pak_"+strings.TrimSuffix(filepath.Base(pak), filepath.Ext(pak)))
		args := []string{"unpack", pak, "-o", unpackDir, "-q"}
		for d := range dirs {
			args = append(args, "-i", d)
		}
		out, err := tools.RunSilent(ctx, workDir, repak, args...)
		if err != nil {
			log.Warn("Failed to unpack pak dirs", "file", pak, "error", err, "output", firstLines(out, 5))
			continue
		}

		uexpFiles := scanner.WalkDir(unpackDir, 12, func(p string) bool {
			return strings.ToLower(filepath.Ext(p)) == ".uexp"
		})
		for _, uexp := range uexpFiles {
			data, err := os.ReadFile(uexp)
			if err != nil {
				continue
			}
			relPath, _ := filepath.Rel(unpackDir, uexp)
			texts := extractUexpStrings(data)
			for _, s := range texts {
				proj.AddEntry(core.StringEntry{
					Source:  s,
					File:    filepath.ToSlash(relPath),
					Context: "Unreal",
				})
			}
			if len(texts) > 0 {
				proj.ExtractedFiles = append(proj.ExtractedFiles, filepath.ToSlash(relPath))
			}
		}
	}
}

// ueMaxUexp caps the number of .uexp files processed per pak to keep
// extraction time bounded on huge paks.
const ueMaxUexp = 8000

// sanitizePakName converts a pak entry path into a safe file name.
func sanitizePakName(p string) string {
	s := strings.ReplaceAll(p, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}

// ── Godot ───────────────────────────────────────────────────────────

type GodotExtractor struct{}

func (e *GodotExtractor) SupportedEngine() string { return "godot" }
func (e *GodotExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "fftools",
	}
}

func (e *GodotExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return err
	}

	// 1. Find .pck files.
	pckFiles := scanner.WalkDir(info.GameRoot, 3, func(p string) bool {
		return strings.ToLower(filepath.Ext(p)) == ".pck"
	})
	if len(pckFiles) == 0 {
		log.Warn("No .pck file found for Godot game")
		return nil
	}

	// 2. Preferred path: recover the project with gdre_tools (converts
	//    binary .res/.scn/.gdc to text .tres/.tscn/.gd), then parse the text
	//    files for strings. This recovers text that is hardcoded in scenes,
	//    resources, and scripts (no .translation files present).
	gdre := gdrePath(reg)
	if gdre == "" {
		log.Warn("gdre_tools.exe not found - falling back to pck extraction (may miss scene/script text)")
		return e.extractPckOnly(ctx, info, proj, reg, workDir, pckFiles)
	}

	recDir := filepath.Join(workDir, "gdre_recovered")
	_ = os.RemoveAll(recDir)
	out, err := tools.RunSilent(ctx, workDir, gdre, "--headless", "--recover="+pckFiles[0], "--output="+recDir)
	if err != nil {
		log.Warn("gdre recovery failed", "error", err, "output", firstLines(out, 3))
		return e.extractPckOnly(ctx, info, proj, reg, workDir, pckFiles)
	}

	// 3. Handle .translation files from the recovered project (same logic
	//    as the pck-only path).
	pckRel, err := filepath.Rel(info.GameRoot, pckFiles[0])
	if err != nil {
		return fmt.Errorf("resolve Godot pack path: %w", err)
	}
	e.exportTranslationFiles(ctx, proj, reg, workDir, recDir, filepath.ToSlash(pckRel))

	// 4. Parse recovered text files (.tres/.gd/.tscn) for hardcoded strings.
	textFiles := scanner.WalkDir(recDir, 12, func(p string) bool {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".tres", ".gd", ".tscn":
			return true
		}
		return false
	})

	skippedEditorAddonFiles := 0
	for _, tf := range textFiles {
		relPath, err := filepath.Rel(recDir, tf)
		if err != nil {
			log.Warn("Could not make Godot text path relative", "file", tf, "error", err)
			continue
		}
		if isGodotEditorAddon(relPath) {
			skippedEditorAddonFiles++
			continue
		}
		data, err := os.ReadFile(tf)
		if err != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)
		texts := extractGodotTextEntries(data)
		for _, entry := range texts {
			identity := sha256.Sum256([]byte(filepath.ToSlash(pckRel) + "\x00" + relPath + "\x00" + entry.Text))
			sourceHash := sha256.Sum256([]byte(entry.Text))
			proj.AddEntry(core.StringEntry{
				ID:        fmt.Sprintf("godot-text:%x", identity[:16]),
				Source:    entry.Text,
				File:      relPath,
				Container: filepath.ToSlash(pckRel),
				Path:      fmt.Sprintf("literal:%x", sourceHash[:8]),
				Context:   "Godot text literal",
				Line:      entry.Line,
			})
		}
		if len(texts) > 0 {
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
		}
	}
	if skippedEditorAddonFiles > 0 {
		log.Info("Skipped Godot editor add-on files", "files", skippedEditorAddonFiles)
	}

	return nil
}

// extractPckOnly is the fallback when gdre_tools is unavailable: extract the
// .pck with fftools and export any .translation files found inside.
func (e *GodotExtractor) extractPckOnly(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, workDir string, pckFiles []string) error {
	fftools := fftoolsPath(reg)
	if fftools == "" {
		return core.NewError("extract", "fftools.exe not found (required for Godot .pck extraction)")
	}

	for _, pck := range pckFiles {
		pckRel, _ := filepath.Rel(info.GameRoot, pck)
		extractDir := filepath.Join(workDir, "pck_"+strings.TrimSuffix(filepath.Base(pck), filepath.Ext(pck)))
		out, err := tools.RunSilent(ctx, workDir, fftools, "pck", "extract", pck, "-o", extractDir)
		if err != nil {
			log.Warn("Failed to extract pck", "file", pck, "error", err, "output", firstLines(out, 5))
			continue
		}
		proj.ExtractedFiles = append(proj.ExtractedFiles, pckRel)

		e.exportTranslationFiles(ctx, proj, reg, workDir, extractDir, filepath.ToSlash(pckRel))
	}

	return nil
}

// exportTranslationFiles matches recovered .translation resources to their
// companion Godot CSVs and adds exact hash-resolved messages to the project.
func (e *GodotExtractor) exportTranslationFiles(ctx context.Context, proj *core.Project, reg core.ToolRegistry, workDir, dir, container string) {
	fftools := fftoolsPath(reg)
	if fftools == "" {
		log.Warn("fftools.exe not found (cannot export .translation files)")
		return
	}

	translationFiles := scanner.WalkDir(dir, 8, func(p string) bool {
		return strings.ToLower(filepath.Ext(p)) == ".translation"
	})
	csvFiles := scanner.WalkDir(dir, 8, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".csv")
	})
	if len(translationFiles) == 0 {
		return
	}
	if len(csvFiles) == 0 {
		log.Warn("Godot .translation resources found without source CSV catalogs", "count", len(translationFiles), "dir", dir)
		return
	}
	outputDir := filepath.Join(workDir, "godot_translation_export")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Warn("Could not create Godot translation export directory", "error", err)
		return
	}

	for _, tf := range translationFiles {
		tfRel, err := filepath.Rel(dir, tf)
		if err != nil {
			log.Warn("Could not make Godot translation path relative", "file", tf, "error", err)
			continue
		}
		tfRel = filepath.ToSlash(tfRel)
		catalog := findGodotCatalog(tf, csvFiles)
		if catalog == "" {
			log.Warn("No unambiguous Godot CSV catalog matches translation resource", "file", tf)
			continue
		}
		sourceLanguage, err := godotCSVSourceLanguage(catalog)
		if err != nil {
			log.Warn("Could not identify a source-language column for Godot catalog", "file", catalog, "error", err)
			continue
		}
		if !strings.EqualFold(sourceLanguage, "EN") {
			log.Warn("Godot catalog has no EN column; using its first language column as source text", "catalog", catalog, "source_language", sourceLanguage)
		}
		containerHash := sha256.Sum256([]byte(container))
		namespace := fmt.Sprintf("%s-%x", sanitizePakName(container), containerHash[:6])
		outJSON := filepath.Join(outputDir, namespace, sanitizePakName(tfRel)+".json")
		out, err := tools.RunSilent(ctx, workDir, fftools, "translation", "export", tf, catalog, "--source-language", sourceLanguage, "-o", outJSON)
		if err != nil {
			log.Warn("fftools could not export Godot translation resource", "file", tf, "catalog", catalog, "error", firstLines(out, 3))
			continue
		}
		data, err := os.ReadFile(outJSON)
		if err != nil {
			log.Warn("Could not read Godot translation export", "file", outJSON, "error", err)
			continue
		}
		var messages []struct {
			ID          string `json:"id"`
			Key         string `json:"key"`
			Source      string `json:"source"`
			Translation string `json:"translation"`
			Locale      string `json:"locale"`
		}
		if err := json.Unmarshal(data, &messages); err != nil {
			log.Warn("Invalid Godot translation export JSON", "file", outJSON, "error", err)
			continue
		}
		added := 0
		for _, message := range messages {
			if message.ID == "" || message.Key == "" || strings.TrimSpace(message.Source) == "" {
				continue
			}
			status := core.StatusUntranslated
			if message.Translation != "" {
				status = core.StatusTranslated
			}
			idPrefix := "godot:" + tfRel + "::"
			if container != "" {
				idPrefix = "godot:" + container + "::" + tfRel + "::"
			}
			proj.AddEntry(core.StringEntry{
				ID:          idPrefix + message.ID,
				Source:      message.Source,
				Translation: message.Translation,
				File:        tfRel,
				Container:   container,
				Path:        message.Key,
				Context:     fmt.Sprintf("Godot translation %s (source %s)", message.Locale, sourceLanguage),
				Status:      status,
			})
			added++
		}
		if added == 0 {
			log.Warn("Godot translation resource had no mapped entries", "file", tf, "catalog", catalog)
			continue
		}
		proj.ExtractedFiles = append(proj.ExtractedFiles, tfRel)
		log.Info("Exported Godot translation entries", "file", tfRel, "locale", messages[0].Locale, "source_language", sourceLanguage, "entries", added)
	}
}

func findGodotCatalog(translationPath string, csvFiles []string) string {
	base := strings.TrimSuffix(filepath.Base(translationPath), filepath.Ext(translationPath))
	stem := strings.TrimSuffix(base, filepath.Ext(base)) // remove locale suffix, e.g. .FA
	translationDir := filepath.Clean(filepath.Dir(translationPath))
	var exact, sameDir []string
	for _, candidate := range csvFiles {
		if !strings.EqualFold(filepath.Dir(candidate), translationDir) {
			continue
		}
		sameDir = append(sameDir, candidate)
		if strings.EqualFold(strings.TrimSuffix(filepath.Base(candidate), filepath.Ext(candidate)), stem) {
			exact = append(exact, candidate)
		}
	}
	if len(exact) == 1 {
		return exact[0]
	}
	if len(exact) == 0 && len(sameDir) == 1 {
		return sameDir[0]
	}
	return ""
}

func godotCSVSourceLanguage(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return "", err
	}
	var first string
	for _, raw := range header {
		name := strings.TrimPrefix(strings.TrimSpace(raw), "\ufeff")
		if strings.EqualFold(name, "key") || name == "" {
			continue
		}
		if strings.EqualFold(name, "EN") {
			return name, nil
		}
		if first == "" {
			first = name
		}
	}
	if first == "" {
		return "", fmt.Errorf("catalog has no language columns")
	}
	return first, nil
}

// gdrePath returns the gdre_tools.exe path from the registry, or "" if
// absent.
func gdrePath(reg core.ToolRegistry) string {
	if tr := toolsRegistry(reg); tr != nil && tr.GDRETools != "" {
		return tr.GDRETools
	}
	return reg.GetPath("gdre_tools")
}

// ── Generic ─────────────────────────────────────────────────────────

type GenericExtractor struct{}

func (e *GenericExtractor) SupportedEngine() string { return "generic" }
func (e *GenericExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: false,
	}
}

func (e *GenericExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry) error {
	// Simple text extraction - find common files and extract strings.
	txtFiles := scanner.WalkDir(info.GameRoot, 3, func(p string) bool {
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".txt" || ext == ".ini" || ext == ".json" || ext == ".csv" || ext == ".xml"
	})

	for _, file := range txtFiles {
		if err := extractFromTextFile(file, proj, info.GameRoot); err != nil {
			log.Warn("Failed to extract from text file", "file", file, "error", err)
		}
	}

	return nil
}

func extractFromTextFile(path string, proj *core.Project, root string) error {
	relPath, _ := filepath.Rel(root, path)
	ext := strings.ToLower(filepath.Ext(path))

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if ext == ".json" {
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err == nil {
			extractFromMap(m, relPath, "", proj)
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
			return nil
		}
	}

	// Simple line-by-line fallback for ini/properties-style files.
	lines := strings.Split(string(data), "\n")
	for lineNum, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if len(val) > 2 && containsLetters(val) {
					proj.AddEntry(core.StringEntry{
						Source: val,
						File:   relPath,
						Path:   strings.TrimSpace(parts[0]),
						Line:   lineNum + 1,
					})
				}
			}
		}
	}

	proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
	return nil
}

func extractFromMap(m map[string]interface{}, file, pathPrefix string, proj *core.Project) {
	for k, v := range m {
		p := k
		if pathPrefix != "" {
			p = pathPrefix + "." + k
		}

		switch val := v.(type) {
		case string:
			if len(val) > 1 && containsLetters(val) {
				proj.AddEntry(core.StringEntry{
					Source: val,
					File:   file,
					Path:   p,
				})
			}
		case map[string]interface{}:
			extractFromMap(val, file, p, proj)
		}
	}
}

func containsLetters(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}
