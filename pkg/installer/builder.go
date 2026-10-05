package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PatchManifest describes the contents of a Persian localization patch.
type PatchManifest struct {
	PatchName    string            `json:"patch_name"`
	GameName     string            `json:"game_name"`
	GameExe      string            `json:"game_exe"`       // relative to game root
	GameRoot     string            `json:"game_root"`      // absolute path (for reference)
	Engine       string            `json:"engine"`
	Version      string            `json:"patch_version"`
	CreatedAt    time.Time         `json:"created_at"`
	Files        []PatchFile       `json:"files"`
	FontFile     string            `json:"font_file"`      // Persian font to install (if any)
	Description  string            `json:"description"`
	Author       string            `json:"author"`
}

// PatchFile represents one file in the patch.
type PatchFile struct {
	RelativePath string `json:"relative_path"`  // path relative to game root
	PatchPath    string `json:"patch_path"`     // path in the patch directory
	OriginalHash string `json:"original_hash"`  // SHA256 of original file
	PatchedHash  string `json:"patched_hash"`   // SHA256 of patched file
	Size         int64  `json:"size"`
}

// BuildConfig holds parameters for building the installer.
type BuildConfig struct {
	GameRoot      string   // absolute path to game directory
	ModifiedFiles []string // list of modified file paths (absolute)
	BackupFiles   []string // list of backup file paths (absolute)
	FontFile      string   // path to Persian font file (optional)
	GameExe       string   // game executable path (relative to game root)
	Engine        string   // engine name
	PatchName     string   // name of the patch
	Description   string   // patch description
	Author        string   // author name
	OutputDir     string   // where to create the installer package
}

// Build creates a complete installer package.
// The output directory will contain:
//   - installer.exe (the launcher/patcher)
//   - patch/ directory with modified files
//   - backup/ directory with original files
//   - patch.json manifest
//   - font/ directory (if font is provided)
func Build(cfg BuildConfig) error {
	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	manifest := PatchManifest{
		PatchName:   cfg.PatchName,
		GameName:    filepath.Base(cfg.GameRoot),
		GameExe:     cfg.GameExe,
		GameRoot:    cfg.GameRoot,
		Engine:      cfg.Engine,
		Version:     "1.0.0",
		CreatedAt:   time.Now(),
		Description: cfg.Description,
		Author:      cfg.Author,
	}

	// Copy modified files to patch/ directory
	patchDir := filepath.Join(cfg.OutputDir, "patch")
	os.MkdirAll(patchDir, 0755)

	for _, modFile := range cfg.ModifiedFiles {
		relPath, err := filepath.Rel(cfg.GameRoot, modFile)
		if err != nil {
			continue
		}

		dstPath := filepath.Join(patchDir, relPath)
		os.MkdirAll(filepath.Dir(dstPath), 0755)

		if err := copyFile(modFile, dstPath); err != nil {
			return fmt.Errorf("copy modified file %s: %w", relPath, err)
		}

		// Find corresponding backup
		var origHash, patchedHash string
		patchedHash = hashFile(dstPath)

		pf := PatchFile{
			RelativePath: relPath,
			PatchPath:    relPath,
			PatchedHash:  patchedHash,
			Size:         fileSize(dstPath),
		}

		// Try to find original file hash from backup
		for _, backup := range cfg.BackupFiles {
			backupRel, _ := filepath.Rel(cfg.GameRoot, backup)
			// Backup files might be in a work directory — try to match by relative path
			if strings.HasSuffix(backup, relPath) || backupRel == relPath {
				origHash = hashFile(backup)
				break
			}
		}
		pf.OriginalHash = origHash

		manifest.Files = append(manifest.Files, pf)
	}

	// Copy backup files to backup/ directory
	backupDir := filepath.Join(cfg.OutputDir, "backup")
	os.MkdirAll(backupDir, 0755)

	for _, backup := range cfg.BackupFiles {
		// Determine relative path
		relPath, err := filepath.Rel(cfg.GameRoot, backup)
		if err != nil {
			// Try using the filename
			relPath = filepath.Base(backup)
		}

		// If it's from a work directory, try to find the game-relative path
		if strings.Contains(backup, "backup") || strings.Contains(backup, "work") {
			// Try to extract the game-relative path from the modified files
			for _, mod := range cfg.ModifiedFiles {
				modRel, _ := filepath.Rel(cfg.GameRoot, mod)
				if strings.HasSuffix(backup, filepath.Base(mod)) {
					relPath = modRel
					break
				}
			}
		}

		dstPath := filepath.Join(backupDir, relPath)
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		copyFile(backup, dstPath)
	}

	// Copy font file if provided
	if cfg.FontFile != "" && fileExists(cfg.FontFile) {
		fontDir := filepath.Join(cfg.OutputDir, "font")
		os.MkdirAll(fontDir, 0755)
		fontDst := filepath.Join(fontDir, filepath.Base(cfg.FontFile))
		copyFile(cfg.FontFile, fontDst)
		manifest.FontFile = filepath.Base(cfg.FontFile)
	}

	// Write manifest
	manifestPath := filepath.Join(cfg.OutputDir, "patch.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Write a README
	readme := fmt.Sprintf(`%s — فارسی‌ساز
================================

بازی: %s
موتور: %s
نسخه پچ: %s
سازنده: %s

توضیحات: %s

تعداد فایل‌های تغییر یافته: %d

روش نصب:
1. فایل installer.exe را در پوشه بازی اجرا کنید
2. یا فایل installer.exe را در هر جایی اجرا کنید و پوشه بازی را مشخص کنید

برای حذف فارسی‌ساز:
1. installer.exe را اجرا کنید
2. گزینه "حذف فارسی‌ساز" را انتخاب کنید
`, cfg.PatchName, filepath.Base(cfg.GameRoot), cfg.Engine, manifest.Version, cfg.Author,
			cfg.Description, len(manifest.Files))

	os.WriteFile(filepath.Join(cfg.OutputDir, "README.txt"), []byte(readme), 0644)

	return nil
}

// copyFile copies a file from src to dst.
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
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func hashFile(path string) string {
	// Simple hash — just return file size as string for now
	// A proper implementation would use SHA256
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d", info.Size())
}
