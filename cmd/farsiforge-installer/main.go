package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PatchManifest matches the installer.PatchManifest structure.
type PatchManifest struct {
	PatchName   string      `json:"patch_name"`
	GameName    string      `json:"game_name"`
	GameExe     string      `json:"game_exe"`
	GameRoot    string      `json:"game_root"`
	Engine      string      `json:"engine"`
	Version     string      `json:"patch_version"`
	CreatedAt   time.Time   `json:"created_at"`
	Files       []PatchFile `json:"files"`
	FontFile    string      `json:"font_file"`
	Description string      `json:"description"`
	Author      string      `json:"author"`
}

type PatchFile struct {
	RelativePath string `json:"relative_path"`
	PatchPath    string `json:"patch_path"`
	OriginalHash string `json:"original_hash"`
	PatchedHash  string `json:"patched_hash"`
	Size         int64  `json:"size"`
}

var manifest *PatchManifest
var installerDir string

func main() {
	// Determine installer directory (where this exe lives)
	exePath, _ := os.Executable()
	installerDir = filepath.Dir(exePath)

	// Load manifest
	manifestPath := filepath.Join(installerDir, "patch.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Println("خطا: فایل patch.json یافت نشد!")
		fmt.Println("Error: patch.json not found!")
		fmt.Printf("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	manifest = &PatchManifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		fmt.Println("خطا: فایل patch.json نامعتبر است!")
		fmt.Println("Error: invalid patch.json!")
		fmt.Printf("\nPress Enter to exit...")
		fmt.Scanln()
		return
	}

	// Check command line args
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--apply", "-a":
			applyPatch()
			return
		case "--uninstall", "-u":
			uninstallPatch()
			return
		case "--launch", "-l":
			launchGame()
			return
		case "--help", "-h":
			showHelp()
			return
		}
	}

	// Interactive menu
	showMenu()
}

func showMenu() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Printf("║  %s\n", pad(manifest.PatchName+" — فارسی‌ساز", 52))
	fmt.Println("╠══════════════════════════════════════════════════════╣")
	fmt.Printf("║  بازی: %s\n", pad(manifest.GameName, 47))
	fmt.Printf("║  موتور: %s\n", pad(manifest.Engine, 46))
	fmt.Printf("║  نسخه: %s\n", pad(manifest.Version, 47))
	fmt.Printf("║  سازنده: %s\n", pad(manifest.Author, 45))
	fmt.Println("╠══════════════════════════════════════════════════════╣")
	fmt.Println("║                                                      ║")
	fmt.Println("║  ۱. نصب فارسی‌ساز          (Install Patch)           ║")
	fmt.Println("║  ۲. اجرای بازی            (Launch Game)             ║")
	fmt.Println("║  ۳. حذف فارسی‌ساز          (Uninstall)               ║")
	fmt.Println("║  ۴. خروج                  (Exit)                    ║")
	fmt.Println("║                                                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Print("\nانتخاب کنید (1-4): ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1":
		applyPatch()
	case "2":
		launchGame()
	case "3":
		uninstallPatch()
	case "4":
		return
	default:
		fmt.Println("انتخاب نامعتبر!")
		showMenu()
	}
}

func applyPatch() {
	fmt.Println("\n──────── نصب فارسی‌ساز ────────")
	fmt.Println("Installing Persian patch...")

	// Determine game root
	gameRoot := manifest.GameRoot
	if gameRoot == "" || !dirExists(gameRoot) {
		// Ask user for game directory
		fmt.Print("\nمسیر پوشه بازی را وارد کنید (Enter game directory path): ")
		var input string
		fmt.Scanln(&input)
		gameRoot = input
	}

	if !dirExists(gameRoot) {
		fmt.Println("خطا: پوشه بازی یافت نشد!")
		fmt.Println("Error: game directory not found!")
		waitExit()
		return
	}

	// Check if already patched
	backupDir := filepath.Join(gameRoot, ".farsiforge_backup")
	if dirExists(backupDir) {
		fmt.Println("\n⚠ فارسی‌ساز قبلاً نصب شده است!")
		fmt.Println("Patch already installed. Uninstall first?")
		fmt.Print("Uninstall and reinstall? (y/n): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			uninstallPatchSilent(gameRoot, backupDir)
		} else {
			waitExit()
			return
		}
	}

	// Create backup directory
	os.MkdirAll(backupDir, 0755)
	fmt.Println("✓ پوشه پشتیبان ایجاد شد (Backup directory created)")

	// Apply each file
	patchDir := filepath.Join(installerDir, "patch")
	successCount := 0
	failCount := 0

	for _, pf := range manifest.Files {
		patchFile := filepath.Join(patchDir, pf.RelativePath)
		gameFile := filepath.Join(gameRoot, pf.RelativePath)

		// Backup original
		if fileExists(gameFile) {
			backupPath := filepath.Join(backupDir, pf.RelativePath)
			os.MkdirAll(filepath.Dir(backupPath), 0755)
			if err := copyFile(gameFile, backupPath); err != nil {
				fmt.Printf("  ⚠ پشتیبانگیری ناموفق: %s\n", pf.RelativePath)
				failCount++
				continue
			}
		}

		// Copy patched file
		if fileExists(patchFile) {
			os.MkdirAll(filepath.Dir(gameFile), 0755)
			if err := copyFile(patchFile, gameFile); err != nil {
				fmt.Printf("  ✗ خطا: %s\n", pf.RelativePath)
				failCount++
			} else {
				fmt.Printf("  ✓ %s\n", pf.RelativePath)
				successCount++
			}
		} else {
			fmt.Printf("  ⚠ فایل پچ یافت نشد: %s\n", pf.RelativePath)
			failCount++
		}
	}

	// Install font if provided
	if manifest.FontFile != "" {
		fontSrc := filepath.Join(installerDir, "font", manifest.FontFile)
		if fileExists(fontSrc) {
			fmt.Println("\nدر حال نصب فونت فارسی... (Installing Persian font...)")
			installFont(fontSrc, gameRoot)
		}
	}

	// Write patch info file
	infoPath := filepath.Join(gameRoot, ".farsiforge_info.json")
	info := map[string]interface{}{
		"patch_name":  manifest.PatchName,
		"version":     manifest.Version,
		"installed":   time.Now().Format(time.RFC3339),
		"file_count":  successCount,
	}
	infoData, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(infoPath, infoData, 0644)

	fmt.Printf("\n═══════════════════════════════════════\n")
	fmt.Printf("  نصب کامل شد! (Installation complete!)\n")
	fmt.Printf("  موفق: %d | ناموفق: %d\n", successCount, failCount)
	fmt.Printf("═══════════════════════════════════════\n")

	if failCount == 0 {
		fmt.Print("\nبازی اجرا شود؟ (Launch game?) (y/n): ")
		var launch string
		fmt.Scanln(&launch)
		if strings.ToLower(launch) == "y" || strings.ToLower(launch) == "yes" {
			launchGame()
			return
		}
	}

	waitExit()
}

func uninstallPatch() {
	gameRoot := manifest.GameRoot
	if gameRoot == "" || !dirExists(gameRoot) {
		fmt.Print("\nمسیر پوشه بازی را وارد کنید: ")
		var input string
		fmt.Scanln(&input)
		gameRoot = input
	}

	backupDir := filepath.Join(gameRoot, ".farsiforge_backup")
	uninstallPatchSilent(gameRoot, backupDir)
	waitExit()
}

func uninstallPatchSilent(gameRoot, backupDir string) {
	if !dirExists(backupDir) {
		fmt.Println("خطا: پشتیبان یافت نشد! (No backup found!)")
		return
	}

	fmt.Println("\nدر حال حذف فارسی‌ساز... (Uninstalling...)")

	// Restore all backed up files
	filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(backupDir, path)
		gameFile := filepath.Join(gameRoot, relPath)

		if err := copyFile(path, gameFile); err != nil {
			fmt.Printf("  ✗ %s\n", relPath)
		} else {
			fmt.Printf("  ✓ %s\n", relPath)
		}
		return nil
	})

	// Remove backup and info files
	os.RemoveAll(backupDir)
	os.Remove(filepath.Join(gameRoot, ".farsiforge_info.json"))

	fmt.Println("\n✓ فارسی‌ساز حذف شد! (Patch uninstalled!)")
}

func launchGame() {
	gameRoot := manifest.GameRoot
	if gameRoot == "" || !dirExists(gameRoot) {
		fmt.Print("\nمسیر پوشه بازی را وارد کنید: ")
		var input string
		fmt.Scanln(&input)
		gameRoot = input
	}

	gameExe := filepath.Join(gameRoot, manifest.GameExe)
	if !fileExists(gameExe) {
		// Try to find the exe
		fmt.Println("خطا: فایل اجرایی بازی یافت نشد!")
		fmt.Printf("Looking for: %s\n", gameExe)
		waitExit()
		return
	}

	fmt.Printf("\nدر حال اجرای بازی... (Launching game...)\n")
	cmd := exec.Command(gameExe)
	cmd.Dir = gameRoot
	if err := cmd.Start(); err != nil {
		fmt.Printf("خطا در اجرای بازی: %v\n", err)
	} else {
		fmt.Println("✓ بازی اجرا شد! (Game launched!)")
	}
}

func installFont(fontPath, gameRoot string) {
	// Try to copy font to game's font directory
	fontDirs := []string{
		filepath.Join(gameRoot, "www", "fonts"),
		filepath.Join(gameRoot, "fonts"),
		filepath.Join(gameRoot, "Data", "Managed", "Fonts"),
		filepath.Join(gameRoot, "Content", "Fonts"),
	}

	for _, dir := range fontDirs {
		if dirExists(dir) {
			dst := filepath.Join(dir, filepath.Base(fontPath))
			copyFile(fontPath, dst)
			fmt.Printf("  ✓ فونت نصب شد: %s\n", dir)
			return
		}
	}

	// Also install system-wide
	if runtime.GOOS == "windows" {
		fontsDir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
		if dirExists(fontsDir) {
			dst := filepath.Join(fontsDir, filepath.Base(fontPath))
			copyFile(fontPath, dst)
			fmt.Printf("  ✓ فونت سیستمی نصب شد\n")
		}
	}
}

func showHelp() {
	fmt.Println("FarsiForge Installer — فارسی‌ساز")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  installer.exe              Interactive menu")
	fmt.Println("  installer.exe --apply      Install patch")
	fmt.Println("  installer.exe --uninstall  Remove patch")
	fmt.Println("  installer.exe --launch     Launch game")
	fmt.Println("  installer.exe --help       Show this help")
}

func waitExit() {
	fmt.Print("\nبرای خروج Enter را فشار دهید... (Press Enter to exit...)")
	fmt.Scanln()
}

// ── Utility functions ────────────────────────────────────────────────

func pad(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	os.MkdirAll(filepath.Dir(dst), 0755)

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
