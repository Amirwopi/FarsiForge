package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ProjectFileName       = "project.json"
	LegacyProjectFileName = ".farsiforge_project.json"
	EnvProjectsDir        = "FARISIFORGE_PROJECTS_DIR"
)

// ProjectFilePath returns the per-game project location outside the game
// installation. FARISIFORGE_PROJECTS_DIR can select a portable/custom store.
func ProjectFilePath(gameRoot string) (string, error) {
	if strings.TrimSpace(gameRoot) == "" {
		return "", fmt.Errorf("game root is required")
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return "", fmt.Errorf("resolve game root: %w", err)
	}
	key := root
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	sum := sha256.Sum256([]byte(key))
	store := os.Getenv(EnvProjectsDir)
	if store == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		store = filepath.Join(configDir, "FarsiForge", "projects")
	}
	return filepath.Join(store, hex.EncodeToString(sum[:16]), ProjectFileName), nil
}

// NewGameProject creates a project with its durable data and work files stored
// under the user-selected project store rather than beside game assets.
func NewGameProject(name, gameRoot, engine string) (*Project, string, error) {
	projectPath, err := ProjectFilePath(gameRoot)
	if err != nil {
		return nil, "", err
	}
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return nil, "", fmt.Errorf("resolve game root: %w", err)
	}
	projectDir := filepath.Dir(projectPath)
	project := NewProject(name, root, engine)
	project.ProjectDir = projectDir
	project.WorkingDir = filepath.Join(projectDir, "work")
	return project, projectPath, nil
}

// LoadGameProject loads a game's FarsiForge project. Existing project files
// stored in the game directory are copied into the external project store;
// the legacy file is retained so this migration never deletes user data.
func LoadGameProject(gameRoot string) (*Project, error) {
	projectPath, err := ProjectFilePath(gameRoot)
	if err != nil {
		return nil, err
	}
	project, err := LoadProject(projectPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		return normalizeProjectPaths(project, gameRoot, projectPath)
	}

	legacyPath := filepath.Join(gameRoot, LegacyProjectFileName)
	project, err = LoadProject(legacyPath)
	if err != nil {
		return nil, err
	}
	project, err = normalizeProjectPaths(project, gameRoot, projectPath)
	if err != nil {
		return nil, err
	}
	if err := project.Save(projectPath); err != nil {
		return nil, fmt.Errorf("migrate legacy project: %w", err)
	}
	return project, nil
}

func normalizeProjectPaths(project *Project, gameRoot, projectPath string) (*Project, error) {
	root, err := canonicalGameRoot(gameRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve game root: %w", err)
	}
	projectDir := filepath.Dir(projectPath)
	if strings.TrimSpace(project.GameRoot) != "" {
		storedRoot, absErr := canonicalGameRoot(project.GameRoot)
		if absErr == nil && !samePath(storedRoot, root) {
			return nil, fmt.Errorf("project belongs to a different game directory: %s", project.GameRoot)
		}
	}
	project.GameRoot = root
	project.ProjectDir = projectDir
	project.WorkingDir = filepath.Join(projectDir, "work")
	return project, nil
}

func canonicalGameRoot(gameRoot string) (string, error) {
	root, err := filepath.Abs(gameRoot)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return filepath.Clean(root), nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	return a == b
}
