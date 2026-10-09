// Package backup provides file backup, restore, and rollback functionality.
//
// Ensures original game files are never modified without a reproducible
// way to revert changes.
package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"farsiforge/pkg/core"
	"farsiforge/pkg/hash"
	"farsiforge/pkg/logging"
)

// BackupManager handles file backups and restores.
type BackupManager struct {
	gameRoot  string
	backupDir string
	log       *logging.Logger
}

// New creates a new BackupManager.
func New(gameRoot, backupDir string) *BackupManager {
	return &BackupManager{
		gameRoot:  gameRoot,
		backupDir: backupDir,
		log:       logging.Default().WithModule("backup"),
	}
}

// EnsureBackupDir creates the backup directory if it doesn't exist.
func (b *BackupManager) EnsureBackupDir() error {
	if err := os.MkdirAll(b.backupDir, 0755); err != nil {
		return core.Wrap("backup", err, "failed to create backup directory")
	}
	return nil
}

// BackupFile creates a backup of a file relative to the game root.
// Returns the path to the backup file and its SHA256 hash.
func (b *BackupManager) BackupFile(relPath string) (string, string, error) {
	cleanPath, err := safeRelativePath(relPath)
	if err != nil {
		return "", "", core.WrapFile("backup", relPath, err, "invalid game-relative path")
	}
	relPath = cleanPath
	if err := b.EnsureBackupDir(); err != nil {
		return "", "", err
	}

	srcPath := filepath.Join(b.gameRoot, relPath)
	if !fileExists(srcPath) {
		return "", "", core.NewError("backup", fmt.Sprintf("source file does not exist: %s", srcPath))
	}

	// Create destination path matching original structure
	dstPath := filepath.Join(b.backupDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", "", core.WrapFile("backup", relPath, err, "failed to create backup subdirectories")
	}

	// If backup already exists, just hash it and return
	if fileExists(dstPath) {
		h, err := hash.File(dstPath)
		if err != nil {
			return dstPath, "", err
		}
		b.log.Debug("Backup already exists", "file", relPath, "hash", h)
		return dstPath, h, nil
	}

	// Calculate hash before copying
	origHash, err := hash.File(srcPath)
	if err != nil {
		return "", "", core.WrapFile("backup", relPath, err, "failed to hash original file")
	}

	// Copy file
	if err := copyFile(srcPath, dstPath); err != nil {
		return "", "", core.WrapFile("backup", relPath, err, "failed to copy file to backup")
	}

	b.log.Info("Created backup", "file", relPath, "hash", origHash)
	return dstPath, origHash, nil
}

// RestoreFile restores a file from backup to the game root.
// Verifies the backup hash before restoring if expectedHash is provided.
func (b *BackupManager) RestoreFile(relPath, expectedHash string) error {
	cleanPath, err := safeRelativePath(relPath)
	if err != nil {
		return core.WrapFile("restore", relPath, err, "invalid game-relative path")
	}
	relPath = cleanPath
	srcPath := filepath.Join(b.backupDir, relPath)
	dstPath := filepath.Join(b.gameRoot, relPath)

	if !fileExists(srcPath) {
		return core.NewError("restore", fmt.Sprintf("backup file does not exist: %s", srcPath))
	}

	if expectedHash != "" {
		match, err := hash.Verify(srcPath, expectedHash)
		if err != nil {
			return core.WrapFile("restore", relPath, err, "failed to verify backup hash")
		}
		if !match {
			return core.NewError("restore", fmt.Sprintf("backup hash mismatch for %s", relPath))
		}
	}

	// Ensure destination directory exists (might have been deleted)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return core.WrapFile("restore", relPath, err, "failed to create game subdirectories")
	}

	// Rename original if exists in case copy fails
	var tmpPath string
	if fileExists(dstPath) {
		tmpPath = dstPath + fmt.Sprintf(".tmp_%d", time.Now().UnixNano())
		if err := os.Rename(dstPath, tmpPath); err != nil {
			return core.WrapFile("restore", relPath, err, "failed to preserve existing game file")
		}
	}

	// Copy backup back to game
	if err := copyFile(srcPath, dstPath); err != nil {
		// Restore failed, try to recover tmp file
		if tmpPath != "" {
			removeErr := os.Remove(dstPath)
			if os.IsNotExist(removeErr) {
				removeErr = nil
			}
			restoreErr := os.Rename(tmpPath, dstPath)
			if removeErr != nil || restoreErr != nil {
				return errors.Join(
					core.WrapFile("restore", relPath, err, "failed to copy backup to game"),
					removeErr,
					restoreErr,
				)
			}
		}
		return core.WrapFile("restore", relPath, err, "failed to copy backup to game")
	}

	// Clean up tmp file
	if tmpPath != "" {
		if err := os.Remove(tmpPath); err != nil {
			return core.WrapFile("restore", relPath, err, "failed to remove temporary original file")
		}
	}

	b.log.Info("Restored file", "file", relPath)
	return nil
}

// RollbackAll restores all backed up files.
func (b *BackupManager) RollbackAll(manifest *core.PatchManifest) error {
	var errs []error
	for _, file := range manifest.Files {
		if err := b.RestoreFile(file.RelativePath, file.OriginalHash); err != nil {
			errs = append(errs, err)
			b.log.Error("Failed to restore", "file", file.RelativePath, "error", err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

func safeRelativePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.ContainsRune(name, 0) || path.IsAbs(name) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := path.Clean(name)
	first := strings.SplitN(clean, "/", 2)[0]
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(first, ":") {
		return "", fmt.Errorf("path must remain within the game directory")
	}
	return filepath.FromSlash(clean), nil
}

// Helper functions

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) (retErr error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Force flush to disk
	return dstFile.Sync()
}
