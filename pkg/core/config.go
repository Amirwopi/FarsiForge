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

	// Server
	ListenAddr string `json:"listen_addr"`
	ListenPort int    `json:"listen_port"`

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
	EnvListenAddr     = "FARISIFORGE_LISTEN_ADDR"
	EnvListenPort     = "FARISIFORGE_LISTEN_PORT"
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
		ListenAddr:         envOr(EnvListenAddr, "127.0.0.1"),
		ListenPort:         envIntOr(EnvListenPort, 7842),
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

	// Steam common locations (only add those that exist on this machine).
	steamCandidates := []string{
		`D:\SteamLibrary\steamapps\common`,
		`C:\Program Files (x86)\Steam\steamapps\common`,
		`E:\SteamLibrary\steamapps\common`,
		`F:\SteamLibrary\steamapps\common`,
		filepath.Join(homeDir, ".steam", "steam", "steamapps", "common"),
	}
	for _, p := range steamCandidates {
		if dirExists(p) {
			paths = append(paths, p)
		}
	}

	// Generic games directory.
	if dirExists(`D:\games`) {
		paths = append(paths, `D:\games`)
	}
	paths = append(paths, filepath.Join(homeDir, "Games"))

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

	// 2. Walk up from CWD looking for go.mod with "farsiforge"
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 10; i++ {
			modPath := filepath.Join(dir, "go.mod")
			if data, err := os.ReadFile(modPath); err == nil {
				if strings.Contains(string(data), "farsiforge") {
					return dir
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 3. Known locations (last resort)
	candidates := []string{
		`D:\FarsiForge`,
		`.`,
	}
	for _, c := range candidates {
		abs, _ := filepath.Abs(c)
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			data, _ := os.ReadFile(filepath.Join(abs, "go.mod"))
			if strings.Contains(string(data), "farsiforge") {
				return abs
			}
		}
	}
	return `D:\FarsiForge`
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
