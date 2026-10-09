// Package scanner provides high-performance filesystem scanning utilities.
// Used for detecting game files, engines, and localization assets.
package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// HasFileWithSuffix checks if a directory contains a file with the given suffix.
func HasFileWithSuffix(dir, suffix string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	suffix = strings.ToLower(suffix)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), suffix) {
			return true
		}
	}
	return false
}

// HasFileWithPrefix checks if a directory contains a file with the given prefix.
func HasFileWithPrefix(dir, prefix string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	prefix = strings.ToLower(prefix)
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), prefix) {
			return true
		}
	}
	return false
}

// FindSubdir finds a subdirectory with the given name (case-insensitive).
func FindSubdir(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// FindDataDir finds a *_Data directory (Unity convention).
func FindDataDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), "_data") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// WalkDir walks up to maxDepth levels and returns all file paths matching a predicate.
func WalkDir(root string, maxDepth int, pred func(string) bool) []string {
	var results []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && pred(path) {
			results = append(results, path)
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return results
}

// FindGameExe looks for the main game executable in the root directory
// given a set of expected patterns.
func FindGameExe(gameRoot string, patterns []string) (exePath, gameName string) {
	entries, err := os.ReadDir(gameRoot)
	if err != nil {
		return "", ""
	}

	for _, pat := range patterns {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(pat)) {
				// Skip obvious non-game executables
				lower := strings.ToLower(name)
				if strings.Contains(lower, "uninstall") ||
					strings.Contains(lower, "crash") ||
					strings.Contains(lower, "redist") ||
					strings.Contains(lower, "setup") {
					continue
				}
				return filepath.Join(gameRoot, name), strings.TrimSuffix(name, filepath.Ext(name))
			}
		}
	}
	return "", ""
}
