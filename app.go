package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/detection"
	"farsiforge/pkg/extract"
	"farsiforge/pkg/inject"
	"farsiforge/pkg/installer"
	"farsiforge/pkg/textfilter"
	"farsiforge/pkg/tools"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	registry *detection.Registry
	toolReg  *tools.Registry
}

// NewApp creates a new App application struct. It uses the portable tool
// registry (NewRegistry with empty string → auto-resolved Tools directory).
func NewApp() *App {
	toolReg, _ := tools.NewRegistry("")
	return &App{
		registry: detection.DefaultRegistry(),
		toolReg:  toolReg,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ── Settings ────────────────────────────────────────────────────────

// Settings is the JSON-serializable settings struct exposed to the frontend.
// It mirrors the user-tunable subset of core.Config.
type Settings struct {
	ToolsDir        string              `json:"tools_dir"`
	OutputDir       string              `json:"output_dir"`
	GameSearchPaths []string            `json:"game_search_paths"`
	DefaultPersian  core.PersianOptions `json:"default_persian_opts"`
	DefaultAuthor   string              `json:"default_author"`
	DefaultLanguage string              `json:"default_language"`
	LogLevel        string              `json:"log_level"`
}

// GetSettings loads and returns the current application settings.
func (a *App) GetSettings() (*Settings, error) {
	cfg, err := core.LoadConfig(core.ConfigPath())
	if err != nil {
		// LoadConfig returns DefaultConfig on missing file, so this is only
		// a parse error.
		return nil, fmt.Errorf("load config: %v", err)
	}
	return &Settings{
		ToolsDir:        cfg.ToolsDir,
		OutputDir:       cfg.OutputDir,
		GameSearchPaths: cfg.GameSearchPaths,
		DefaultPersian:  cfg.DefaultPersianOpts,
		DefaultAuthor:   cfg.DefaultAuthor,
		DefaultLanguage: cfg.DefaultLanguage,
		LogLevel:        cfg.LogLevel,
	}, nil
}

// SaveSettings persists the given settings via the core config API.
func (a *App) SaveSettings(s *Settings) error {
	if s == nil {
		return fmt.Errorf("settings are nil")
	}
	cfg, err := core.LoadConfig(core.ConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %v", err)
	}
	cfg.ToolsDir = s.ToolsDir
	cfg.OutputDir = s.OutputDir
	cfg.GameSearchPaths = s.GameSearchPaths
	cfg.DefaultPersianOpts = s.DefaultPersian
	cfg.DefaultAuthor = s.DefaultAuthor
	cfg.DefaultLanguage = s.DefaultLanguage
	cfg.LogLevel = s.LogLevel
	if err := core.SaveConfig(cfg, core.ConfigPath()); err != nil {
		return fmt.Errorf("save config: %v", err)
	}
	return nil
}

// ── Tool status ─────────────────────────────────────────────────────

// ToolStatus reports the presence and version of the tools FarsiForge relies
// on. All fields are JSON-serializable for the Wails frontend.
type ToolStatus struct {
	ToolsDir      string          `json:"tools_dir"`
	Python        string          `json:"python"`
	PythonVersion string          `json:"python_version"`
	PythonOK      bool            `json:"python_ok"`
	UnityPy       bool            `json:"unity_py"`
	FFTools       string          `json:"ff_tools"`
	FFToolsOK     bool            `json:"ff_tools_ok"`
	PatcherExe    string          `json:"patcher_exe"`
	PatcherOK     bool            `json:"patcher_ok"`
	Available     map[string]bool `json:"available"`
}

// GetToolStatus reports the presence/version of python, UnityPy, fftools, and
// the patcher executable.
func (a *App) GetToolStatus() *ToolStatus {
	reg := a.toolReg
	if reg == nil {
		reg, _ = tools.NewRegistry("")
		a.toolReg = reg
	}
	status := &ToolStatus{
		ToolsDir:      reg.RootDir,
		Python:        reg.Python,
		PythonVersion: reg.PythonVersion,
		PythonOK:      reg.IsAvailable("python"),
		UnityPy:       reg.IsAvailable("py_UnityPy"),
		FFTools:       reg.FFTools,
		FFToolsOK:     reg.IsAvailable("fftools"),
		Available:     reg.Available,
	}
	// Resolve the patcher exe: Tools\patcher\FarsiForgePatcher.exe.
	patcherExe := filepath.Join(reg.RootDir, "patcher", "FarsiForgePatcher.exe")
	status.PatcherExe = patcherExe
	status.PatcherOK = fileExists(patcherExe)
	return status
}

// InstallPythonPackage runs `pip install <name>` silently via pkg/tools and
// returns the combined output/error text.
func (a *App) InstallPythonPackage(name string) (string, error) {
	reg := a.toolReg
	if reg == nil || reg.Python == "" {
		return "", fmt.Errorf("Python is not available; install Python first")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("package name is required")
	}
	// Use `python -m pip install` so it targets the same interpreter.
	out, err := tools.RunSilent(a.ctx, "", reg.Python, "-m", "pip", "install", name)
	if err != nil {
		return out, fmt.Errorf("pip install failed: %v\n%s", err, out)
	}
	// Refresh the registry so newly installed packages are detected.
	reg, _ = tools.NewRegistry(reg.RootDir)
	a.toolReg = reg
	return out, nil
}

// ── Detection / extraction / injection ──────────────────────────────

// DetectEngine runs the engine detection.
func (a *App) DetectEngine(path string) (*core.GameInfo, error) {
	gameRoot, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve game directory: %w", err)
	}
	res, err := a.registry.Detect(gameRoot)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %v", err)
	}

	dataPath := ""
	if len(res.DataPaths) > 0 {
		dataPath = res.DataPaths[0]
	}
	gameName := strings.TrimSpace(res.GameName)
	if gameName == "" {
		gameName = filepath.Base(gameRoot)
	}

	return &core.GameInfo{
		Engine:     res.Engine,
		Backend:    res.Backend,
		Version:    res.Version,
		GameName:   gameName,
		GameExe:    res.GameExe,
		GameRoot:   gameRoot,
		DataPath:   dataPath,
		Confidence: res.Confidence,
		Evidence:   res.Evidence,
		Metadata:   res.Metadata,
	}, nil
}

// Extract runs the extraction pipeline.
func (a *App) Extract(info *core.GameInfo) (int, error) {
	if info == nil {
		return 0, fmt.Errorf("game information is required")
	}
	previous, loadErr := core.LoadGameProject(info.GameRoot)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return 0, fmt.Errorf("load existing project: %w", loadErr)
	}
	gameName := strings.TrimSpace(info.GameName)
	if gameName == "" {
		gameName = filepath.Base(filepath.Clean(info.GameRoot))
	}
	proj, projectPath, err := core.NewGameProject(gameName+" Localization", info.GameRoot, info.Engine)
	if err != nil {
		return 0, err
	}
	proj.GameName = gameName
	proj.GameExe = info.GameExe
	proj.Backend = info.Backend
	proj.Version = info.Version

	if err := extract.Run(a.ctx, info, proj, a.toolReg); err != nil {
		return 0, fmt.Errorf("extraction failed: %v", err)
	}
	if previous != nil {
		proj.MergeTranslations(previous)
	}
	if err := proj.Save(projectPath); err != nil {
		return 0, fmt.Errorf("failed to save project: %v", err)
	}

	return len(proj.Entries), nil
}

// Inject runs the injection pipeline.
func (a *App) Inject(info *core.GameInfo) (int, error) {
	if info == nil {
		return 0, fmt.Errorf("game information is required")
	}
	proj, err := core.LoadGameProject(info.GameRoot)
	if err != nil {
		return 0, fmt.Errorf("load project: %w", err)
	}
	if proj.Engine != info.Engine {
		return 0, fmt.Errorf("project engine %q does not match detected engine %q", proj.Engine, info.Engine)
	}
	projectPath, err := core.ProjectFilePath(info.GameRoot)
	if err != nil {
		return 0, err
	}
	proj.ModifiedFiles = nil
	proj.ModifiedFileHashes = nil
	if err := proj.Save(projectPath); err != nil {
		return 0, fmt.Errorf("clear previous staged injection state: %w", err)
	}

	opts := proj.PersianOpts
	if opts == (core.PersianOptions{}) {
		opts = core.DefaultPersianOptions()
	}

	modified, err := inject.Run(a.ctx, info, proj, a.toolReg, opts)
	if err != nil {
		return 0, fmt.Errorf("injection failed: %v", err)
	}
	if err := proj.Save(projectPath); err != nil {
		return 0, fmt.Errorf("save injection results: %w", err)
	}

	return len(modified), nil
}

// LaunchGame starts the game executable silently (no console window flash).
func (a *App) LaunchGame(gameRoot, exeName string) error {
	exePath := filepath.Join(gameRoot, exeName)
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("game executable not found: %s", exePath)
	}
	// Use RunSilent to avoid a console window flash on Windows.
	_, err := tools.RunSilent(a.ctx, gameRoot, exePath)
	if err != nil {
		return fmt.Errorf("failed to launch game: %v", err)
	}
	return nil
}

// SelectDirectory opens a directory selection dialog
func (a *App) SelectDirectory() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "انتخاب پوشه بازی",
	})
}

// GetProject loads the project
func (a *App) GetProject(gameRoot string) (*core.Project, error) {
	return core.LoadGameProject(gameRoot)
}

// SearchEntries loads the project saved under gameRoot and returns the
// entries matching the query (case-insensitive substring across ID, Source,
// Translation, File, and Notes) and the optional status filter (a status
// value or "qa" for entries with QA notes; "" disables filtering).
func (a *App) SearchEntries(gameRoot, query, status string) ([]core.StringEntry, error) {
	proj, err := a.GetProject(gameRoot)
	if err != nil {
		return nil, err
	}
	return proj.SearchEntries(query, status), nil
}

// SaveTranslations saves translations
func (a *App) SaveTranslations(gameRoot string, entries []core.StringEntry) error {
	proj, err := core.LoadGameProject(gameRoot)
	if err != nil {
		return err
	}
	projectIndexes := make(map[string]int, len(proj.Entries))
	for i, entry := range proj.Entries {
		if entry.ID == "" {
			return fmt.Errorf("project entry at index %d is missing its stable ID", i)
		}
		if _, exists := projectIndexes[entry.ID]; exists {
			return fmt.Errorf("project contains duplicate entry ID %q", entry.ID)
		}
		projectIndexes[entry.ID] = i
	}
	byID := make(map[string]core.StringEntry, len(entries))
	for _, entry := range entries {
		if entry.ID == "" {
			return fmt.Errorf("translation entry is missing its stable ID")
		}
		if _, exists := byID[entry.ID]; exists {
			return fmt.Errorf("duplicate translation entry ID %q", entry.ID)
		}
		if _, exists := projectIndexes[entry.ID]; !exists {
			return fmt.Errorf("translation entry %q is not part of the current project; reload before saving", entry.ID)
		}
		switch entry.Status {
		case core.StatusUntranslated, core.StatusTranslated, core.StatusApproved, core.StatusSkipped:
		default:
			return fmt.Errorf("invalid translation status %q for entry %q", entry.Status, entry.ID)
		}
		byID[entry.ID] = entry
	}
	for id, incoming := range byID {
		i := projectIndexes[id]
		proj.Entries[i].Translation = incoming.Translation
		proj.Entries[i].Status = incoming.Status
		if proj.Entries[i].Translation == "" && proj.Entries[i].Status != core.StatusSkipped {
			proj.Entries[i].Status = core.StatusUntranslated
		} else if proj.Entries[i].Translation != "" && proj.Entries[i].Status == core.StatusUntranslated {
			proj.Entries[i].Status = core.StatusTranslated
		}
		proj.Entries[i].Notes = textfilter.QANotes(textfilter.QA(proj.Entries[i].Source, proj.Entries[i].Translation))
	}
	projectPath, err := core.ProjectFilePath(gameRoot)
	if err != nil {
		return err
	}
	return proj.Save(projectPath)
}

// ── Patcher build ───────────────────────────────────────────────────

// BuildPatcherResult describes the produced patch package.
type BuildPatcherResult struct {
	OutputDir   string `json:"output_dir"`
	PatchFile   string `json:"patch_file"`
	PatcherExe  string `json:"patcher_exe"`
	TargetCount int    `json:"target_count"`
}

// BuildPatcher builds the final FFP1 patch package using pkg/ffpatch and
// stages FarsiForgePatcher.exe alongside it. The modified files are taken
// from the project's ModifiedFiles list; their patched content is read from
// the project working directory.
func (a *App) BuildPatcher(gameRoot string, credits string) (*BuildPatcherResult, error) {
	proj, err := core.LoadGameProject(gameRoot)
	if err != nil {
		return nil, fmt.Errorf("load project: %v", err)
	}

	// Resolve the patcher exe from the tool registry.
	reg := a.toolReg
	if reg == nil {
		reg, _ = tools.NewRegistry("")
		a.toolReg = reg
	}
	patcherExe := filepath.Join(reg.RootDir, "patcher", "FarsiForgePatcher.exe")

	// Collect patch targets: each modified file's patched version lives in
	// the project working directory (injectors write outputs there). We map
	// the game-relative path to the patched file in workDir.
	workDir := proj.WorkingDir
	if workDir == "" {
		return nil, fmt.Errorf("project working directory is not configured")
	}

	var targets []installer.PatchTarget
	for _, mf := range proj.ModifiedFiles {
		gamePath := filepath.ToSlash(mf)
		if filepath.IsAbs(mf) || strings.Contains(gamePath, "../") || gamePath == ".." {
			return nil, fmt.Errorf("invalid modified game path %q", mf)
		}
		patchedFile := filepath.Join(workDir, "out", filepath.FromSlash(gamePath))
		if !fileExists(patchedFile) {
			return nil, fmt.Errorf("staged patched file missing for %q: %s", mf, patchedFile)
		}
		originalHash := proj.ModifiedFileHashes[mf]
		if originalHash == "" {
			return nil, fmt.Errorf("staged source hash missing for %q; inject again before building the patch", mf)
		}
		targets = append(targets, installer.PatchTarget{
			GamePath:       gamePath,
			PatchedFile:    patchedFile,
			OriginalSHA256: originalHash,
		})
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no patched files found to build a patch (run injection first)")
	}

	outputDir := filepath.Join(proj.ProjectDir, "dist")
	cfg := installer.BuildConfig{
		GameRoot:    gameRoot,
		Targets:     targets,
		PatcherExe:  patcherExe,
		GameExe:     proj.GameExe,
		Engine:      proj.Engine,
		PatchName:   proj.GameName + " — فارسی‌ساز",
		Description: "FarsiForge Persian localization patch",
		Author:      credits,
		OutputDir:   outputDir,
	}

	res, err := installer.Build(cfg)
	if err != nil {
		return nil, fmt.Errorf("build patcher: %v", err)
	}

	return &BuildPatcherResult{
		OutputDir:   res.OutputDir,
		PatchFile:   res.PatchFile,
		PatcherExe:  res.PatcherExe,
		TargetCount: res.TargetCount,
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────

// fileExists is a local helper (app package) to avoid importing scanner here.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
