package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Basic sanity checks — values should be non-empty
	if cfg.ToolsDir == "" {
		t.Error("ToolsDir should not be empty")
	}
	if cfg.OutputDir == "" {
		t.Error("OutputDir should not be empty")
	}
	if cfg.TempDir == "" {
		t.Error("TempDir should not be empty")
	}
	if cfg.ListenAddr == "" {
		t.Error("ListenAddr should not be empty")
	}
	if cfg.ListenPort <= 0 {
		t.Error("ListenPort should be positive")
	}
	if cfg.MaxFileSizeMB <= 0 {
		t.Error("MaxFileSizeMB should be positive")
	}
	if cfg.MaxScanDepth <= 0 {
		t.Error("MaxScanDepth should be positive")
	}
	if cfg.PythonExe == "" {
		t.Error("PythonExe should not be empty")
	}
	if cfg.ProjectRoot == "" {
		t.Error("ProjectRoot should not be empty")
	}
	if len(cfg.GameSearchPaths) == 0 {
		t.Error("GameSearchPaths should not be empty")
	}
}

func TestDefaultConfig_PythonExeEnvOverride(t *testing.T) {
	t.Setenv(EnvPythonExe, "/custom/python3.11")
	cfg := DefaultConfig()
	if cfg.PythonExe != "/custom/python3.11" {
		t.Errorf("PythonExe = %q, want %q", cfg.PythonExe, "/custom/python3.11")
	}
}

func TestDefaultConfig_ListenPortEnvOverride(t *testing.T) {
	t.Setenv(EnvListenPort, "9999")
	cfg := DefaultConfig()
	if cfg.ListenPort != 9999 {
		t.Errorf("ListenPort = %d, want 9999", cfg.ListenPort)
	}
}

func TestDefaultConfig_GameSearchPathEnvOverride(t *testing.T) {
	t.Setenv(EnvGameSearchPath, `C:\MyGames;D:\MoreGames`)
	cfg := DefaultConfig()

	found := false
	for _, p := range cfg.GameSearchPaths {
		if p == `C:\MyGames` || p == `D:\MoreGames` {
			found = true
		}
	}
	if !found {
		t.Error("GameSearchPaths should contain env-provided paths")
	}
}

func TestDefaultConfig_ProjectRootEnvOverride(t *testing.T) {
	// Create a temp dir with a fake go.mod
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module farsiforge\n\ngo 1.26.0\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvProjectRoot, dir)
	cfg := DefaultConfig()
	if cfg.ProjectRoot != dir {
		t.Errorf("ProjectRoot = %q, want %q", cfg.ProjectRoot, dir)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	cfg.DefaultAuthor = "TestAuthor"

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.DefaultAuthor != "TestAuthor" {
		t.Errorf("DefaultAuthor = %q, want %q", loaded.DefaultAuthor, "TestAuthor")
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadConfig should not error on missing file: %v", err)
	}
	// Should return defaults
	if cfg.ListenPort <= 0 {
		t.Error("should return default config with valid ListenPort")
	}
}
