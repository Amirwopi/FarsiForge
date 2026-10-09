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

func TestFindProjectRootUsesExactModuleName(t *testing.T) {
	t.Run("finds nested FarsiForge checkout", func(t *testing.T) {
		root := t.TempDir()
		nested := filepath.Join(root, "work")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module farsiforge\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findProjectRootFrom(nested); got != root {
			t.Fatalf("findProjectRootFrom() = %q, want %q", got, root)
		}
	})

	t.Run("does not match similarly named modules", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/Amirwopi/farsiforge-tools\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findProjectRootFrom(root); got != "" {
			t.Fatalf("findProjectRootFrom() = %q, want no match", got)
		}
	})
}

func TestDefaultGameSearchPathsUsesConfiguredSteamLibraries(t *testing.T) {
	temp := t.TempDir()
	lib := filepath.Join(temp, "library")
	common := filepath.Join(lib, "steamapps", "common")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"STEAM_PATH", "ProgramFiles", "ProgramFiles(x86)"} {
		t.Setenv(key, "")
	}
	t.Setenv("STEAM_LIBRARY_PATHS", lib+string(os.PathListSeparator)+lib)
	got := defaultGameSearchPaths(filepath.Join(temp, "home"))
	count := 0
	for _, candidate := range got {
		if candidate == common {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("defaultGameSearchPaths() = %#v, expected %q exactly once", got, common)
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
	if cfg.MaxScanDepth <= 0 {
		t.Error("missing config should return defaults")
	}
}
