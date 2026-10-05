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
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		ToolsDir:  findToolsDir(),
		OutputDir: filepath.Join(findProjectRoot(), "output"),
		TempDir:   filepath.Join(os.TempDir(), "farsiforge"),
		GameSearchPaths: []string{
			`D:\SteamLibrary\steamapps\common`,
			`D:\games`,
			`C:\Program Files (x86)\Steam\steamapps\common`,
			filepath.Join(homeDir, "Games"),
		},
		ListenAddr:         "127.0.0.1",
		ListenPort:         7842,
		DefaultPersianOpts: DefaultPersianOptions(),
		DefaultAuthor:      "FarsiForge",
		DefaultLanguage:    "fa-IR",
		LogLevel:           "info",
		MaxFileSizeMB:      100,
		MaxScanDepth:       6,
	}
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
func findProjectRoot() string {
	// Check common locations
	candidates := []string{
		`D:\FarsiForge`,
		`.`,
	}
	for _, c := range candidates {
		abs, _ := filepath.Abs(c)
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			// Verify it's the FarsiForge go.mod
			data, _ := os.ReadFile(filepath.Join(abs, "go.mod"))
			if strings.Contains(string(data), "farsiforge") {
				return abs
			}
		}
	}
	return `D:\FarsiForge`
}

// findToolsDir locates the Tools directory.
func findToolsDir() string {
	root := findProjectRoot()
	toolsDir := filepath.Join(root, "Tools")
	if info, err := os.Stat(toolsDir); err == nil && info.IsDir() {
		return toolsDir
	}
	return toolsDir // Return it even if missing, will be created
}
