package core

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProjectFilePathIsOutsideGameRoot(t *testing.T) {
	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(EnvProjectsDir, store)

	path, err := ProjectFilePath(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(gameRoot, path)
	if err != nil {
		t.Fatal(err)
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		t.Fatalf("project path %q is inside game root %q", path, gameRoot)
	}
}

func TestProjectFilePathNormalizesWindowsCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows paths are case-insensitive")
	}
	gameRoot := t.TempDir()
	t.Setenv(EnvProjectsDir, t.TempDir())

	lowerPath, err := ProjectFilePath(strings.ToLower(gameRoot))
	if err != nil {
		t.Fatal(err)
	}
	upperPath, err := ProjectFilePath(strings.ToUpper(gameRoot))
	if err != nil {
		t.Fatal(err)
	}
	if lowerPath != upperPath {
		t.Fatalf("case variants resolved to different project files: %q != %q", lowerPath, upperPath)
	}
}

func TestLoadGameProjectAcceptsStoredSymlinkAlias(t *testing.T) {
	gameRoot := t.TempDir()
	linkRoot := filepath.Join(t.TempDir(), "game-link")
	if err := os.Symlink(gameRoot, linkRoot); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	store := t.TempDir()
	t.Setenv(EnvProjectsDir, store)

	project, projectPath, err := NewGameProject("linked", linkRoot, "unity")
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(project.GameRoot, gameRoot) {
		t.Fatalf("NewGameProject stored root %q, want canonical root %q", project.GameRoot, gameRoot)
	}
	if err := project.Save(projectPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadGameProject(gameRoot)
	if err != nil {
		t.Fatalf("LoadGameProject(canonical root): %v", err)
	}
	if !samePath(loaded.GameRoot, gameRoot) {
		t.Fatalf("loaded root = %q, want %q", loaded.GameRoot, gameRoot)
	}
}

func TestSaveReplacesExistingProjectFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.json")
	project := NewProject("before", t.TempDir(), "unity")
	if err := project.Save(path); err != nil {
		t.Fatal(err)
	}
	project.Name = "after"
	if err := project.Save(path); err != nil {
		t.Fatalf("replace existing project: %v", err)
	}
	loaded, err := LoadProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "after" {
		t.Fatalf("saved name = %q, want after", loaded.Name)
	}
}

func TestLoadGameProjectMigratesLegacyWithoutDeletingIt(t *testing.T) {
	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(EnvProjectsDir, store)

	legacyPath := filepath.Join(gameRoot, LegacyProjectFileName)
	legacy := NewProject("legacy", gameRoot, "unity")
	legacy.WorkingDir = filepath.Join(gameRoot, "work")
	legacy.AddEntry(StringEntry{ID: "stable-id", File: "Data/assets", Path: "field", Source: "Start", Translation: "شروع", Status: StatusApproved})
	if err := legacy.Save(legacyPath); err != nil {
		t.Fatal(err)
	}

	project, err := LoadGameProject(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	projectPath, err := ProjectFilePath(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(projectPath); err != nil {
		t.Fatalf("migrated project not found: %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy project should be retained: %v", err)
	}
	if project.Entries[0].Translation != "شروع" || project.Entries[0].Status != StatusApproved {
		t.Fatalf("migration lost translation: %+v", project.Entries[0])
	}
	if rel, err := filepath.Rel(gameRoot, project.WorkingDir); err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)) {
		t.Fatalf("working directory %q should be outside game root (rel=%q, err=%v)", project.WorkingDir, rel, err)
	}
}

func TestLoadGameProjectRejectsProjectForDifferentRoot(t *testing.T) {
	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(EnvProjectsDir, store)

	project, _, err := NewGameProject("wrong", t.TempDir(), "unity")
	if err != nil {
		t.Fatal(err)
	}
	projectPath, err := ProjectFilePath(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := project.Save(projectPath); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGameProject(gameRoot)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadGameProject() error = %v, want project-root mismatch", err)
	}
}
