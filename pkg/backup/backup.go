// Package backup provides file backup, restore, and rollback functionality.
//
// Ensures original game files are never modified without a reproducible
// way to revert changes.
package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		os.Rename(dstPath, tmpPath)
	}

	// Copy backup back to game
	if err := copyFile(srcPath, dstPath); err != nil {
		// Restore failed, try to recover tmp file
		if tmpPath != "" {
			os.Rename(tmpPath, dstPath)
		}
		return core.WrapFile("restore", relPath, err, "failed to copy backup to game")
	}

	// Clean up tmp file
	if tmpPath != "" {
		os.Remove(tmpPath)
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
		return core.NewError("rollback", fmt.Sprintf("rollback completed with %d errors", len(errs)))
	}
	return nil
}

// Helper functions

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	
	// Force flush to disk
	return dstFile.Sync()
}
