package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/detection"
	"farsiforge/pkg/extract"
	"farsiforge/pkg/inject"
	"farsiforge/pkg/installer"
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
	res, err := a.registry.Detect(path)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %v", err)
	}

	dataPath := ""
	if len(res.DataPaths) > 0 {
		dataPath = res.DataPaths[0]
	}

	return &core.GameInfo{
		Engine:     res.Engine,
		Backend:    res.Backend,
		Version:    res.Version,
		GameName:   res.GameName,
		GameExe:    res.GameExe,
		GameRoot:   path,
		DataPath:   dataPath,
		Confidence: res.Confidence,
		Evidence:   res.Evidence,
		Metadata:   res.Metadata,
	}, nil
}

// Extract runs the extraction pipeline.
func (a *App) Extract(info *core.GameInfo) (int, error) {
	proj := core.NewProject("Local Project", info.GameRoot, info.Engine)

	if err := extract.Run(a.ctx, info, proj, a.toolReg); err != nil {
		return 0, fmt.Errorf("extraction failed: %v", err)
	}

	// Ensure the project directory exists before saving.
	projDir := info.GameRoot
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create project directory: %v", err)
	}

	projPath := filepath.Join(projDir, ".farsiforge_project.json")
	if err := proj.Save(projPath); err != nil {
		return 0, fmt.Errorf("failed to save project: %v", err)
	}

	return len(proj.Entries), nil
}

// Inject runs the injection pipeline.
func (a *App) Inject(info *core.GameInfo) (int, error) {
	projPath := filepath.Join(info.GameRoot, ".farsiforge_project.json")
	proj, err := core.LoadProject(projPath)
	if err != nil {
		proj = core.NewProject("Local Project", info.GameRoot, info.Engine)
	}

	opts := core.DefaultPersianOptions()

	modified, err := inject.Run(a.ctx, info, proj, a.toolReg, opts)
	if err != nil {
		return 0, fmt.Errorf("injection failed: %v", err)
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
	projPath := filepath.Join(gameRoot, ".farsiforge_project.json")
	return core.LoadProject(projPath)
}

// SaveTranslations saves translations
func (a *App) SaveTranslations(gameRoot string, entries []core.StringEntry) error {
	projPath := filepath.Join(gameRoot, ".farsiforge_project.json")
	proj, err := core.LoadProject(projPath)
	if err != nil {
		return err
	}
	proj.Entries = entries
	return proj.Save(projPath)
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
	projPath := filepath.Join(gameRoot, ".farsiforge_project.json")
	proj, err := core.LoadProject(projPath)
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
		workDir = filepath.Join(filepath.Dir(projPath), "work")
	}

	var targets []installer.PatchTarget
	for _, mf := range proj.ModifiedFiles {
		gamePath := mf
		// Modified files may be stored as game-relative or absolute paths.
		patchedFile := filepath.Join(workDir, filepath.Base(mf))
		if !filepath.IsAbs(mf) {
			// If the patched copy exists in workDir, use it; otherwise assume
			// the file was patched in-place in the game tree.
			if fileExists(patchedFile) {
				targets = append(targets, installer.PatchTarget{
					GamePath:    gamePath,
					PatchedFile: patchedFile,
				})
				continue
			}
			patchedFile = filepath.Join(gameRoot, mf)
		}
		if !fileExists(patchedFile) {
			// Skip missing files rather than failing the whole build.
			continue
		}
		targets = append(targets, installer.PatchTarget{
			GamePath:    gamePath,
			PatchedFile: patchedFile,
		})
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no patched files found to build a patch (run injection first)")
	}

	outputDir := filepath.Join(gameRoot, "FarsiForge_Patch")
	cfg := installer.BuildConfig{
		GameRoot:    gameRoot,
		Targets:     targets,
		PatcherExe:  patcherExe,
		GameExe:     proj.GameName,
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

// _ keeps encoding/json imported for future use without an unused-import
// error if Settings serialization helpers are added later.
var _ = json.Marshal
