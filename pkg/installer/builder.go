// Package installer builds the end-user Persian localization patch package.
//
// The real deliverable is an FFP1 (FarsiForge Patch v1) binary patch file
// produced by pkg/ffpatch (the Go writer, byte-identical to the C# reader in
// patcher/FarsiForgePatcher.cs) plus a copy of FarsiForgePatcher.exe staged
// next to it. The patcher applies the .ffp1 file to the game directory.
//
// See patcher/FORMAT.md for the FFP1 format specification.
package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"farsiforge/pkg/ffpatch"
)

// PatchTarget describes one file to include in the patch.
type PatchTarget struct {
	// GamePath is the game-RELATIVE path of the target file, e.g.
	// "Game_Data/resources.assets". This is what the patcher writes back to.
	GamePath string `json:"game_path"`
	// PatchedFile is the absolute path to the already-patched file whose
	// content will be embedded as a ModeReplace payload.
	PatchedFile string `json:"patched_file"`
	// OriginalFile is the unmodified game asset used for FFP1 v2 compatibility checks.
	// If empty, GameRoot/GamePath is used.
	OriginalFile string `json:"original_file,omitempty"`
	// OriginalSHA256 is captured when injection stages the replacement. It
	// prevents building a patch from stale staged output after a game update.
	OriginalSHA256 string `json:"original_sha256,omitempty"`
}

// BuildConfig holds parameters for building the installer/patch package.
type BuildConfig struct {
	// GameRoot is the absolute path to the (original) game directory. Used
	// only to compute default game-relative paths when GamePath is empty.
	GameRoot string `json:"game_root"`

	// Targets are the files to embed in the FFP1 patch.
	Targets []PatchTarget `json:"targets"`

	// PatcherExe is the absolute path to FarsiForgePatcher.exe. It is copied
	// into the output directory next to the .ffp1 patch. If empty, the
	// patcher binary is omitted (and a warning is returned).
	PatcherExe string `json:"patcher_exe"`

	// FontFile is an optional Persian font to install alongside the patch.
	FontFile string `json:"font_file,omitempty"`

	// Metadata for the FFP1 header.
	GameExe     string `json:"game_exe"`
	Engine      string `json:"engine"`
	PatchName   string `json:"patch_name"`
	Description string `json:"description"`
	Author      string `json:"author"`

	// OutputDir is where the package is created.
	OutputDir string `json:"output_dir"`
}

// BuildResult describes the produced package.
type BuildResult struct {
	OutputDir     string `json:"output_dir"`
	PatchFile     string `json:"patch_file"`
	PatcherExe    string `json:"patcher_exe"`
	TargetCount   int    `json:"target_count"`
	FontInstalled bool   `json:"font_installed"`
}

// PatchFileName is the name of the FFP1 patch file inside the package.
const PatchFileName = "FarsiForgePatch.ffp1"

// PatcherExeName is the name of the staged patcher binary.
const PatcherExeName = "FarsiForgePatcher.exe"

// Build creates a complete patch package: an FFP1 binary patch written with
// pkg/ffpatch (ModeReplace per target) plus a copy of FarsiForgePatcher.exe,
// an optional font, and a README. It returns a BuildResult describing the
// outputs.
func Build(cfg BuildConfig) (*BuildResult, error) {
	if cfg.OutputDir == "" {
		return nil, fmt.Errorf("installer: output_dir is required")
	}
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("installer: no patch targets provided")
	}
	if err := validateBuildConfig(cfg); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("installer: create output dir: %w", err)
	}

	result := &BuildResult{OutputDir: cfg.OutputDir, TargetCount: len(cfg.Targets)}

	// 1. Write the FFP1 patch file.
	patchPath := filepath.Join(cfg.OutputDir, PatchFileName)
	if err := writeFFP1Patch(patchPath, cfg); err != nil {
		return result, fmt.Errorf("installer: write FFP1 patch: %w", err)
	}
	result.PatchFile = patchPath

	// 2. Stage the patcher binary alongside the patch.
	if cfg.PatcherExe != "" {
		if !fileExists(cfg.PatcherExe) {
			return result, fmt.Errorf("installer: patcher executable not found: %s", cfg.PatcherExe)
		}
		dst := filepath.Join(cfg.OutputDir, PatcherExeName)
		if err := copyFile(cfg.PatcherExe, dst); err != nil {
			return result, fmt.Errorf("installer: stage patcher exe: %w", err)
		}
		result.PatcherExe = dst
	}

	// 3. Copy font file if provided.
	if cfg.FontFile != "" {
		if !fileExists(cfg.FontFile) {
			return result, fmt.Errorf("installer: font file not found: %s", cfg.FontFile)
		}
		fontDir := filepath.Join(cfg.OutputDir, "font")
		if err := os.MkdirAll(fontDir, 0o755); err != nil {
			return result, fmt.Errorf("installer: create font directory: %w", err)
		}
		fontDst := filepath.Join(fontDir, filepath.Base(cfg.FontFile))
		if err := copyFile(cfg.FontFile, fontDst); err != nil {
			return result, fmt.Errorf("installer: stage font: %w", err)
		}
		result.FontInstalled = true
	}

	// 4. Write a README.
	if err := writeReadme(cfg, result); err != nil {
		return result, fmt.Errorf("installer: write README: %w", err)
	}

	return result, nil
}

func validateBuildConfig(cfg BuildConfig) error {
	if strings.TrimSpace(cfg.GameRoot) == "" {
		return fmt.Errorf("installer: game_root is required to verify patch targets")
	}
	if cfg.PatcherExe != "" && !fileExists(cfg.PatcherExe) {
		return fmt.Errorf("installer: patcher executable not found: %s", cfg.PatcherExe)
	}
	if cfg.FontFile != "" && !fileExists(cfg.FontFile) {
		return fmt.Errorf("installer: font file not found: %s", cfg.FontFile)
	}
	seenGamePaths := make(map[string]struct{}, len(cfg.Targets))
	for i, target := range cfg.Targets {
		if !fileExists(target.PatchedFile) {
			return fmt.Errorf("installer: target %d file not found: %s", i, target.PatchedFile)
		}
		gamePath := target.GamePath
		if gamePath == "" && cfg.GameRoot != "" && filepath.IsAbs(target.PatchedFile) {
			var err error
			gamePath, err = filepath.Rel(cfg.GameRoot, target.PatchedFile)
			if err != nil {
				return fmt.Errorf("installer: resolve target %d game path: %w", i, err)
			}
		}
		if gamePath == "" {
			gamePath = filepath.Base(target.PatchedFile)
		}
		cleanGamePath, err := cleanGamePath(gamePath)
		if err != nil {
			return fmt.Errorf("installer: invalid target %d game path %q: %w", i, gamePath, err)
		}
		pathKey := strings.ToLower(cleanGamePath)
		if _, exists := seenGamePaths[pathKey]; exists {
			return fmt.Errorf("installer: duplicate target game path %q", cleanGamePath)
		}
		seenGamePaths[pathKey] = struct{}{}
		originalPath := target.OriginalFile
		if originalPath == "" {
			originalPath = filepath.Join(cfg.GameRoot, filepath.FromSlash(gamePath))
		}
		if !fileExists(originalPath) {
			return fmt.Errorf("installer: original target %d file not found: %s", i, originalPath)
		}
		if target.OriginalSHA256 != "" {
			if len(target.OriginalSHA256) != sha256.Size*2 {
				return fmt.Errorf("installer: target %d has an invalid original SHA-256", i)
			}
			if _, err := hex.DecodeString(target.OriginalSHA256); err != nil {
				return fmt.Errorf("installer: target %d has an invalid original SHA-256: %w", i, err)
			}
			original, err := os.Open(originalPath)
			if err != nil {
				return fmt.Errorf("installer: open original target %d: %w", i, err)
			}
			hasher := sha256.New()
			_, copyErr := io.Copy(hasher, original)
			closeErr := original.Close()
			if copyErr != nil {
				return fmt.Errorf("installer: hash original target %d: %w", i, copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("installer: close original target %d: %w", i, closeErr)
			}
			if !strings.EqualFold(target.OriginalSHA256, fmt.Sprintf("%x", hasher.Sum(nil))) {
				return fmt.Errorf("installer: original target %q changed after injection; inject again before building the patch", cleanGamePath)
			}
		}
	}
	return nil
}

// writeFFP1Patch streams an FFP1 patch to patchPath. Each target is added as
// a ModeReplace target with a single embedded payload (the full patched file
// content). Final sha256/size are computed automatically by the writer.
func writeFFP1Patch(patchPath string, cfg BuildConfig) (retErr error) {
	f, err := os.Create(patchPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); retErr == nil && err != nil {
			retErr = fmt.Errorf("close patch file: %w", err)
		}
	}()

	w, err := ffpatch.NewWriter(f)
	if err != nil {
		return err
	}
	w.SetMetadata(ffpatch.Metadata{
		PatchName:    orDefault(cfg.PatchName, "FarsiForge Patch"),
		GameName:     filepath.Base(orDefault(cfg.GameRoot, ".")),
		GameExe:      cfg.GameExe,
		Engine:       cfg.Engine,
		PatchVersion: "1.0.0",
		Author:       orDefault(cfg.Author, "FarsiForge"),
		Description:  cfg.Description,
		CreatedAt:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})

	for _, t := range cfg.Targets {
		gamePath := t.GamePath
		if gamePath == "" && cfg.GameRoot != "" && filepath.IsAbs(t.PatchedFile) {
			gamePath, _ = filepath.Rel(cfg.GameRoot, t.PatchedFile)
		}
		if gamePath == "" {
			gamePath = filepath.Base(t.PatchedFile)
		}
		// Normalize to forward slashes for cross-platform consistency.
		gamePath = filepath.ToSlash(gamePath)
		gamePath, err = cleanGamePath(gamePath)
		if err != nil {
			return fmt.Errorf("invalid game path %q: %w", t.GamePath, err)
		}

		data, err := os.ReadFile(t.PatchedFile)
		if err != nil {
			return fmt.Errorf("read patched file %s: %w", t.PatchedFile, err)
		}

		target, err := w.AddTarget(gamePath, ffpatch.ModeReplace)
		if err != nil {
			return err
		}
		originalPath := t.OriginalFile
		if originalPath == "" {
			originalPath = filepath.Join(cfg.GameRoot, filepath.FromSlash(gamePath))
		}
		original, err := os.Open(originalPath)
		if err != nil {
			return fmt.Errorf("open original target %s: %w", gamePath, err)
		}
		stat, statErr := original.Stat()
		if statErr != nil {
			_ = original.Close()
			return fmt.Errorf("stat original target %s: %w", gamePath, statErr)
		}
		hasher := sha256.New()
		if _, err := io.Copy(hasher, original); err != nil {
			_ = original.Close()
			return fmt.Errorf("hash original target %s: %w", gamePath, err)
		}
		if err := original.Close(); err != nil {
			return fmt.Errorf("close original target %s: %w", gamePath, err)
		}
		if t.OriginalSHA256 != "" && !strings.EqualFold(t.OriginalSHA256, fmt.Sprintf("%x", hasher.Sum(nil))) {
			return fmt.Errorf("original target %s changed after injection; inject again before building the patch", gamePath)
		}
		if err := target.SetBaseHash(hasher.Sum(nil), stat.Size()); err != nil {
			return err
		}
		target.AddPayload(gamePath, data)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("finalize patch: %w", err)
	}
	return nil
}

func cleanGamePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.ContainsRune(name, 0) || path.IsAbs(name) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path must stay inside the game directory")
	}
	for _, component := range strings.Split(clean, "/") {
		if component == "" || component == "." || component == ".." ||
			strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return "", fmt.Errorf("path contains a component with unsafe Windows semantics")
		}
		for _, r := range component {
			if r < 32 || strings.ContainsRune(`<>:"|?*`, r) {
				return "", fmt.Errorf("path contains a character unavailable in Windows filenames")
			}
		}
		deviceName := strings.ToUpper(strings.SplitN(component, ".", 2)[0])
		if isWindowsDeviceName(deviceName) {
			return "", fmt.Errorf("path contains a reserved Windows device name")
		}
	}
	return clean, nil
}

func isWindowsDeviceName(name string) bool {
	switch name {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

// writeReadme writes a bilingual README.txt into the output directory.
func writeReadme(cfg BuildConfig, result *BuildResult) error {
	readme := fmt.Sprintf(`%s — فارسی‌ساز
================================

بازی: %s
موتور: %s
نسخه پچ: 1.0.0
سازنده: %s

توضیحات: %s
تعداد فایل‌های تغییر یافته: %d

روش نصب:
1. فایل %s و FarsiForgePatch.ffp1 را در کنار هم نگه دارید
2. %s را اجرا کنید و پوشه بازی را مشخص کنید
3. یا %s را در پوشه بازی کپی کرده و اجرا کنید

برای حذف فارسی‌ساز:
1. %s را اجرا کنید
2. گزینه "حذف فارسی‌ساز" را انتخاب کنید

---
Game: %s
Engine: %s
Patch version: 1.0.0
Author: %s
Description: %s
Modified files: %d

Install:
1. Keep %s and FarsiForgePatch.ffp1 together
2. Run %s and select the game folder
3. Or copy %s into the game folder and run it

Uninstall:
1. Run %s
2. Choose "Uninstall"
`,
		orDefault(cfg.PatchName, "FarsiForge Patch"),
		filepath.Base(orDefault(cfg.GameRoot, ".")), cfg.Engine,
		orDefault(cfg.Author, "FarsiForge"), cfg.Description, result.TargetCount,
		PatcherExeName, PatcherExeName, PatcherExeName, PatcherExeName,
		filepath.Base(orDefault(cfg.GameRoot, ".")), cfg.Engine,
		orDefault(cfg.Author, "FarsiForge"), cfg.Description, result.TargetCount,
		PatcherExeName, PatcherExeName, PatcherExeName, PatcherExeName,
	)

	return os.WriteFile(filepath.Join(cfg.OutputDir, "README.txt"), []byte(readme), 0o644)
}

// ── helpers ─────────────────────────────────────────────────────────

func copyFile(src, dst string) (retErr error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); retErr == nil && err != nil {
			retErr = fmt.Errorf("close source file: %w", err)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); retErr == nil && err != nil {
			retErr = fmt.Errorf("close destination file: %w", err)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
