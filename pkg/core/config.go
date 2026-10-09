package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the FarsiForge configuration.
type Config struct {
	// Paths
	ToolsDir  string `json:"tools_dir"`
	OutputDir string `json:"output_dir"`
	TempDir   string `json:"temp_dir"`

	// Game search paths
	GameSearchPaths []string `json:"game_search_paths"`

	// Defaults
	DefaultPersianOpts PersianOptions `json:"default_persian_opts"`
	DefaultAuthor      string         `json:"default_author"`
	DefaultLanguage    string         `json:"default_language"`

	// Logging
	LogLevel string `json:"log_level"` // "debug", "info", "warn", "error"
	LogFile  string `json:"log_file"`

	// Security
	MaxFileSizeMB int `json:"max_file_size_mb"` // Max file size to load into memory
	MaxScanDepth  int `json:"max_scan_depth"`   // Max directory depth for scanning

	// ── New optional fields (de-hardcoded, env-overridable) ──
	// PythonExe is the Python executable name/path used by external tool
	// scripts. Defaults to "python" on Windows, "python3" elsewhere.
	// Override with the FARISIFORGE_PYTHON env var.
	PythonExe string `json:"python_exe,omitempty"`

	// ProjectRoot is the FarsiForge project root directory.
	// If empty, it is auto-detected. Override with FARISIFORGE_ROOT.
	ProjectRoot string `json:"project_root,omitempty"`
}

// Environment variable names used for de-hardcoding config values.
const (
	EnvProjectRoot    = "FARISIFORGE_ROOT"
	EnvToolsDir       = "FARISIFORGE_TOOLS_DIR"
	EnvOutputDir      = "FARISIFORGE_OUTPUT_DIR"
	EnvTempDir        = "FARISIFORGE_TEMP_DIR"
	EnvPythonExe      = "FARISIFORGE_PYTHON"
	EnvLogLevel       = "FARISIFORGE_LOG_LEVEL"
	EnvMaxFileSizeMB  = "FARISIFORGE_MAX_FILE_SIZE_MB"
	EnvMaxScanDepth   = "FARISIFORGE_MAX_SCAN_DEPTH"
	EnvGameSearchPath = "FARISIFORGE_GAME_SEARCH_PATH"
)

// defaultPythonExe returns the default Python executable name for the
// current platform.
func defaultPythonExe() string {
	if v := os.Getenv(EnvPythonExe); v != "" {
		return v
	}
	if isWindows() {
		return "python"
	}
	return "python3"
}

// isWindows returns true on Windows.
func isWindows() bool {
	return filepath.Separator == '\\' && os.PathSeparator == ';'
}

// envOr returns the env var value if set and non-empty, otherwise the
// provided fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	root := findProjectRoot()

	// Build game search paths from env + sensible defaults.
	gamePaths := defaultGameSearchPaths(homeDir)
	if extra := os.Getenv(EnvGameSearchPath); extra != "" {
		gamePaths = append(gamePaths, filepath.SplitList(extra)...)
	}

	return Config{
		ToolsDir:           findToolsDir(),
		OutputDir:          envOr(EnvOutputDir, filepath.Join(root, "output")),
		TempDir:            envOr(EnvTempDir, filepath.Join(os.TempDir(), "farsiforge")),
		GameSearchPaths:    gamePaths,
		DefaultPersianOpts: DefaultPersianOptions(),
		DefaultAuthor:      "FarsiForge",
		DefaultLanguage:    "fa-IR",
		LogLevel:           envOr(EnvLogLevel, "info"),
		MaxFileSizeMB:      envIntOr(EnvMaxFileSizeMB, 100),
		MaxScanDepth:       envIntOr(EnvMaxScanDepth, 6),
		PythonExe:          defaultPythonExe(),
		ProjectRoot:        root,
	}
}

// defaultGameSearchPaths returns the default game search paths, derived
// from the home directory and common Steam/Epic locations without
// hardcoding a single absolute path.
func defaultGameSearchPaths(homeDir string) []string {
	var paths []string
	seen := make(map[string]struct{})
	add := func(p string) {
		if p == "" {
			return
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return
		}
		key := filepath.Clean(abs)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if dirExists(key) {
			paths = append(paths, key)
		}
	}

	steamRoots := []string{
		os.Getenv("STEAM_PATH"),
		filepath.Join(homeDir, ".steam", "steam"),
		filepath.Join(homeDir, ".local", "share", "Steam"),
	}
	if windows := os.Getenv("ProgramFiles(x86)"); windows != "" {
		steamRoots = append(steamRoots, filepath.Join(windows, "Steam"))
	}
	if windows := os.Getenv("ProgramFiles"); windows != "" {
		steamRoots = append(steamRoots, filepath.Join(windows, "Steam"))
	}

	for _, library := range filepath.SplitList(os.Getenv("STEAM_LIBRARY_PATHS")) {
		add(filepath.Join(library, "steamapps", "common"))
	}
	for _, root := range steamRoots {
		add(filepath.Join(root, "steamapps", "common"))
	}
	if homeDir != "" {
		gamesDir, err := filepath.Abs(filepath.Join(homeDir, "Games"))
		if err == nil {
			if _, exists := seen[filepath.Clean(gamesDir)]; !exists {
				paths = append(paths, filepath.Clean(gamesDir))
			}
		}
	}
	return paths
}

// envIntOr returns the env var value parsed as int, or the fallback.
func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}

// dirExists is a local helper to avoid importing the scanner package
// (which would create a circular dependency: scanner has no dep on core,
// but we keep core self-contained).
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// LoadConfig loads configuration from a JSON file.
// If the file doesn't exist, returns DefaultConfig.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// SaveConfig writes configuration to a JSON file.
func SaveConfig(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// ConfigPath returns the default config file path.
func ConfigPath() string {
	// Look for config next to the executable first
	exe, _ := os.Executable()
	if exe != "" {
		configPath := filepath.Join(filepath.Dir(exe), "farsiforge.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Then in the project root
	root := findProjectRoot()
	if root != "" {
		return filepath.Join(root, "configs", "farsiforge.json")
	}

	// Fallback to home directory
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".farsiforge", "config.json")
}

// findProjectRoot tries to find the FarsiForge project root directory.
// It checks the FARISIFORGE_ROOT env var first, then walks up from the
// current working directory looking for a go.mod containing "farsiforge",
// then falls back to known locations.
func findProjectRoot() string {
	// 1. Env var override
	if v := os.Getenv(EnvProjectRoot); v != "" {
		if dirExists(filepath.Join(v, "go.mod")) || dirExists(v) {
			return v
		}
	}

	// 2. Search from the working directory and executable location. This also
	// supports packaged builds started outside the project working directory.
	if cwd, err := os.Getwd(); err == nil {
		if root := findProjectRootFrom(cwd); root != "" {
			return root
		}
	}
	if exe, err := os.Executable(); err == nil {
		if root := findProjectRootFrom(filepath.Dir(exe)); root != "" {
			return root
		}
	}
	return ""
}

func findProjectRootFrom(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 2 && fields[0] == "module" && fields[1] == "farsiforge" {
					return dir
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// findToolsDir locates the Tools directory.
// It checks the FARISIFORGE_TOOLS_DIR env var first, then derives from
// the project root.
func findToolsDir() string {
	if v := os.Getenv(EnvToolsDir); v != "" {
		return v
	}
	root := findProjectRoot()
	toolsDir := filepath.Join(root, "Tools")
	if info, err := os.Stat(toolsDir); err == nil && info.IsDir() {
		return toolsDir
	}
	return toolsDir // Return it even if missing, will be created
}
