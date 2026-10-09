package ffpatch

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cliExe returns the path to the built FarsiForgePatcherCli.exe, or "" if it
// cannot be found / csc unavailable. The build is produced by patcher/build.ps1.
func cliExe() string {
	candidates := []string{
		filepath.Join("..", "..", "patcher", "bin", "FarsiForgePatcherCli.exe"),
		filepath.Join("..", "..", "Tools", "patcher", "FarsiForgePatcherCli.exe"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return ""
}

// TestCrossValidation builds a patch with the Go writer against a temp "game"
// dir, runs the C# CLI to apply it to a copy, verifies the patched files +
// .ffbak backups + sha256 match, then runs --uninstall and verifies
// restoration. This proves Go-writer <-> C#-reader format conformance.
func TestCrossValidation(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	tmp := t.TempDir()
	gameDir := filepath.Join(tmp, "game")
	patchFile := filepath.Join(tmp, "test.ffpatch")

	// 1. set up a fake "original" game layout with pre-existing target files.
	if err := os.MkdirAll(filepath.Join(gameDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	// game exe so the CLI validates the game folder
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("EXE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// original config.txt (will be backed up)
	origConfig := []byte("ORIGINAL config content here")
	if err := os.WriteFile(filepath.Join(gameDir, "config.txt"), origConfig, 0o644); err != nil {
		t.Fatal(err)
	}
	// original assets.bin (will be backed up)
	origAssets := bytes.Repeat([]byte{0xAA}, 256)
	if err := os.WriteFile(filepath.Join(gameDir, "data", "assets.bin"), origAssets, 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. build the patch with the Go writer.
	patchedConfig := []byte("name=فارسی\nversion=1.0\nنصب شد")
	patchedAssets := bytes.Repeat([]byte{0xBB}, 300) // different size than original

	f, err := os.Create(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{
		PatchName:    "CrossVal Patch",
		GameName:     "TestGame",
		GameExe:      "TestGame.exe",
		Engine:       "Unity",
		PatchVersion: "1.0.0",
		Author:       "go-test",
		Description:  "cross-validation",
	})
	t1, _ := w.AddTarget("config.txt", ModeReplace)
	t1.AddPayload("config.txt", patchedConfig)
	t2, _ := w.AddTarget("data/assets.bin", ModeReplace)
	t2.AddPayload("data/assets.bin", patchedAssets)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// 3. run the C# CLI --info to confirm it can read the Go-written header.
	infoCmd := exec.Command(cli, "--info", patchFile)
	infoOut, err := infoCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--info failed: %v\n%s", err, infoOut)
	}
	if !strings.Contains(string(infoOut), "CrossVal Patch") || !strings.Contains(string(infoOut), "TestGame") {
		t.Errorf("--info output missing expected metadata:\n%s", infoOut)
	}
	if !strings.Contains(string(infoOut), "config.txt") || !strings.Contains(string(infoOut), "data/assets.bin") {
		t.Errorf("--info output missing targets:\n%s", infoOut)
	}

	// 4. apply the patch with the C# CLI.
	applyCmd := exec.Command(cli, patchFile, gameDir)
	applyOut, err := applyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, applyOut)
	}

	// 5. verify patched content + .ffbak backups.
	gotConfig, err := os.ReadFile(filepath.Join(gameDir, "config.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotConfig, patchedConfig) {
		t.Errorf("config.txt not patched correctly: got %q want %q", gotConfig, patchedConfig)
	}
	gotAssets, err := os.ReadFile(filepath.Join(gameDir, "data", "assets.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotAssets, patchedAssets) {
		t.Errorf("assets.bin not patched correctly: got %d bytes want %d bytes", len(gotAssets), len(patchedAssets))
	}

	// .ffbak should exist and contain originals
	bakConfig, err := os.ReadFile(filepath.Join(gameDir, "config.txt.ffbak"))
	if err != nil {
		t.Errorf("config.txt.ffbak missing: %v", err)
	} else if !bytes.Equal(bakConfig, origConfig) {
		t.Errorf("config.txt.ffbak content wrong")
	}
	bakAssets, err := os.ReadFile(filepath.Join(gameDir, "data", "assets.bin.ffbak"))
	if err != nil {
		t.Errorf("assets.bin.ffbak missing: %v", err)
	} else if !bytes.Equal(bakAssets, origAssets) {
		t.Errorf("assets.bin.ffbak content wrong")
	}

	// sha256 of patched files must match what the writer recorded
	shaCfg := sha256.Sum256(patchedConfig)
	shaGot := sha256.Sum256(gotConfig)
	if !bytes.Equal(shaCfg[:], shaGot[:]) {
		t.Errorf("config.txt sha mismatch")
	}

	// 6. uninstall and verify restoration.
	unCmd := exec.Command(cli, "--uninstall", patchFile, gameDir)
	unOut, err := unCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("uninstall failed: %v\n%s", err, unOut)
	}
	restoredConfig, err := os.ReadFile(filepath.Join(gameDir, "config.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredConfig, origConfig) {
		t.Errorf("config.txt not restored: got %q want %q", restoredConfig, origConfig)
	}
	restoredAssets, err := os.ReadFile(filepath.Join(gameDir, "data", "assets.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restoredAssets, origAssets) {
		t.Errorf("assets.bin not restored")
	}
	// .ffbak should be gone after uninstall
	if _, err := os.Stat(filepath.Join(gameDir, "config.txt.ffbak")); !os.IsNotExist(err) {
		t.Errorf("config.txt.ffbak should be removed after uninstall")
	}
}

// TestCrossValidationCopyFromBase validates a mode=0 rebuild target with a
// copy-from-base record across the Go writer and C# reader.
func TestCrossValidationCopyFromBase(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}

	tmp := t.TempDir()
	gameDir := filepath.Join(tmp, "game")
	patchFile := filepath.Join(tmp, "test.ffpatch")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("EXE"), 0o644); err != nil {
		t.Fatal(err)
	}
	// base file: 128 bytes 0..127
	baseData := make([]byte, 128)
	for i := range baseData {
		baseData[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "base_source.bin"), baseData, 0o644); err != nil {
		t.Fatal(err)
	}

	// rebuilt.bin = first 64 bytes of base_source.bin (mode=0, copy-from-base)
	rebuilt := baseData[:64]
	rebuiltMd5 := md5.Sum(rebuilt)
	rebuiltSha := sha256.Sum256(rebuilt)

	f, err := os.Create(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := NewWriter(f)
	w.SetMetadata(Metadata{PatchName: "rebuild", GameName: "TestGame", GameExe: "TestGame.exe"})
	t1, _ := w.AddTarget("rebuilt.bin", ModeRebuild)
	t1.AddCopyFromBase("rebuilt:copy", "base_source.bin", 0, int64(len(rebuilt)), rebuiltMd5[:])
	t1.SetFinalHash(rebuiltSha[:], int64(len(rebuilt)))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// apply
	applyCmd := exec.Command(cli, patchFile, gameDir)
	if out, err := applyCmd.CombinedOutput(); err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(gameDir, "rebuilt.bin"))
	if err != nil {
		t.Fatalf("rebuilt.bin missing: %v", err)
	}
	if !bytes.Equal(got, rebuilt) {
		t.Errorf("rebuilt.bin content wrong: got %d bytes want %d bytes", len(got), len(rebuilt))
	}
}

func TestPatcherRejectsUnsafeTargetPath(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}

	patchPath := filepath.Join(t.TempDir(), "unsafe.ffpatch")
	f, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(f)
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	_, err = w.AddTarget("../outside.txt", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(cli, "--info", patchPath).CombinedOutput()
	if err == nil {
		t.Fatalf("patcher accepted an unsafe target path: %s", out)
	}
}
