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

func raceHarnessExe(t *testing.T, cli string) string {
	t.Helper()
	candidates := []string{
		`C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe`,
		`C:\Windows\Microsoft.NET\Framework\v4.0.30319\csc.exe`,
	}
	var csc string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			csc = candidate
			break
		}
	}
	if csc == "" {
		t.Skip("csc.exe not found; run patcher/build.ps1 on Windows")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "PatchRaceHarness.cs")
	exe := filepath.Join(dir, "PatchRaceHarness.exe")
	code := `using System;
using System.IO;
using FarsiForgePatcher;
public static class PatchRaceHarness {
    public static int Main(string[] args) {
        try {
            using (FileStream stream = File.OpenRead(args[1]))
            using (BinaryReader reader = new BinaryReader(stream)) {
                PatchInfo info = PatchJob.ReadHeader(reader);
                Action<string> log = delegate(string message) {
                    if (message == args[5]) {
                        string path = Path.Combine(args[2], args[3]);
                        if (args[0] == "stale-temp") path += ".ffnew";
                        File.WriteAllText(path, args[4]);
                    }
                    if (args.Length > 9 && message == args[6]) {
                        string path = Path.Combine(args[2], args[7]);
                        if (args[8] == "temp") path += ".ffnew";
                        File.WriteAllText(path, args[9]);
                    }
                };
                if (args[0] == "apply" || args[0] == "stale-temp")
                    PatchJob.ApplyAll(info, args[1], args[2], null, log);
                else
                    PatchJob.UninstallAll(info, args[2], log);
            }
            return 0;
        } catch (Exception ex) {
            Console.Error.WriteLine(ex.ToString());
            return 2;
        }
    }
}`
	if err := os.WriteFile(source, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(csc, "/nologo", "/target:exe", "/out:"+exe, "/reference:"+cli, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile patch race harness: %v\n%s", err, output)
	}
	cliBytes, err := os.ReadFile(cli)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(cli)), cliBytes, 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
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
	baseConfigHash := sha256.Sum256(origConfig)
	if err := t1.SetBaseHash(baseConfigHash[:], int64(len(origConfig))); err != nil {
		t.Fatal(err)
	}
	t1.AddPayload("config.txt", patchedConfig)
	t2, _ := w.AddTarget("data/assets.bin", ModeReplace)
	baseAssetsHash := sha256.Sum256(origAssets)
	if err := t2.SetBaseHash(baseAssetsHash[:], int64(len(origAssets))); err != nil {
		t.Fatal(err)
	}
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

func TestPatcherRejectsCopyFromBaseThroughReparsePoint(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	tmp := t.TempDir()
	gameDir := filepath.Join(tmp, "game")
	outsideDir := filepath.Join(tmp, "outside")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := []byte("outside game root")
	if err := os.WriteFile(filepath.Join(outsideDir, "source.bin"), secret, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(gameDir, "outside-link")
	if err := os.Symlink(outsideDir, link); err != nil {
		t.Skipf("cannot create directory symlink on this host: %v", err)
	}
	patchPath := filepath.Join(tmp, "reparse.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		patchFile.Close()
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{PatchName: "Reparse test", GameName: "TestGame"})
	target, err := w.AddTarget("created.bin", ModeRebuild)
	if err != nil {
		patchFile.Close()
		t.Fatal(err)
	}
	digest := md5.Sum(secret)
	finalHash := sha256.Sum256(secret)
	target.AddCopyFromBase("external-copy", filepath.ToSlash(filepath.Join("outside-link", "source.bin")), 0, int64(len(secret)), digest[:])
	target.SetFinalHash(finalHash[:], int64(len(secret)))
	if err := w.Close(); err != nil {
		patchFile.Close()
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cli, patchPath, gameDir)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("patcher accepted copy-from-base through a directory symlink:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "created.bin")); !os.IsNotExist(err) {
		t.Fatalf("outside data was staged into the game: stat error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "created.bin.ffnew")); !os.IsNotExist(err) {
		t.Fatalf("temporary output remains after rejection: stat error=%v", err)
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

func TestPatcherRejectsDuplicateAndWindowsSpecialTargetPaths(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	for _, tc := range []struct {
		name  string
		paths []string
	}{
		{name: "case-insensitive alias", paths: []string{"Data/file.bin", `data\FILE.BIN`}},
		{name: "alternate data stream", paths: []string{"Data/file.bin:stream"}},
		{name: "reserved device name", paths: []string{"CON/settings.txt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patchPath := filepath.Join(t.TempDir(), "invalid-path.ffpatch")
			patchFile, err := os.Create(patchPath)
			if err != nil {
				t.Fatal(err)
			}
			w, err := NewWriter(patchFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.paths {
				if _, err := w.AddTarget(name, ModeReplace); err != nil {
					t.Fatal(err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if err := patchFile.Close(); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(cli, "--info", patchPath).CombinedOutput(); err == nil {
				t.Fatalf("patcher accepted invalid target paths: %s", output)
			}
		})
	}
}

func TestPatcherV2RejectsWrongGameFileAndPreservesExistingBackup(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	base := []byte("expected game build")
	patched := []byte("localized build")
	patchPath := filepath.Join(t.TempDir(), "versioned.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	target, err := w.AddTarget("Data/file.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256(base)
	if err := target.SetBaseHash(baseHash[:], int64(len(base))); err != nil {
		t.Fatal(err)
	}
	target.AddPayload("Data/file.bin", patched)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	wrongGame := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wrongGame, "Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	wrong := []byte("different version")
	targetPath := filepath.Join(wrongGame, "Data", "file.bin")
	if err := os.WriteFile(targetPath, wrong, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wrongGame, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cli, patchPath, wrongGame).CombinedOutput(); err == nil {
		t.Fatalf("v2 patch accepted a different game build: %s", out)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wrong) {
		t.Fatalf("wrong-version target was modified: %q", got)
	}
	backupPath := targetPath + ".ffbak"
	backup := []byte("pre-existing backup")
	if err := os.WriteFile(backupPath, backup, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cli, patchPath, wrongGame).CombinedOutput(); err == nil {
		t.Fatalf("patch overwrote an existing backup: %s", out)
	}
	gotBackup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBackup, backup) {
		t.Fatalf("existing backup changed: %q", gotBackup)
	}
}

func TestPatcherV2RollsBackEarlierTargetsWhenLaterTargetFails(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}

	originalA := []byte("original A")
	patchedA := []byte("modified A")
	originalB := []byte("original B")
	patchPath := filepath.Join(t.TempDir(), "rollback.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})

	targetA, err := w.AddTarget("a.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseAHash := sha256.Sum256(originalA)
	if err := targetA.SetBaseHash(baseAHash[:], int64(len(originalA))); err != nil {
		t.Fatal(err)
	}
	targetA.AddPayload("a.bin", patchedA)

	targetB, err := w.AddTarget("b.bin", ModeRebuild)
	if err != nil {
		t.Fatal(err)
	}
	baseBHash := sha256.Sum256(originalB)
	if err := targetB.SetBaseHash(baseBHash[:], int64(len(originalB))); err != nil {
		t.Fatal(err)
	}
	baseAMD5 := md5.Sum(originalA)
	targetB.AddCopyFromBase("b.bin:a.bin", "a.bin", 0, int64(len(originalA)), baseAMD5[:])
	targetB.SetFinalHash(baseAHash[:], int64(len(originalA)))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{"a.bin": originalA, "b.bin": originalB} {
		if err := os.WriteFile(filepath.Join(gameDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command(cli, patchPath, gameDir).CombinedOutput(); err == nil {
		t.Fatalf("patch unexpectedly succeeded after a later copy-from-base mismatch: %s", out)
	}
	for name, want := range map[string][]byte{"a.bin": originalA, "b.bin": originalB} {
		got, err := os.ReadFile(filepath.Join(gameDir, name))
		if err != nil {
			t.Fatalf("read restored %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s after rollback = %q, want %q", name, got, want)
		}
		for _, suffix := range []string{".ffbak", ".ffnew"} {
			if _, err := os.Stat(filepath.Join(gameDir, name+suffix)); !os.IsNotExist(err) {
				t.Errorf("temporary %s%s remains after rollback: %v", name, suffix, err)
			}
		}
	}
}

func TestPatcherV2PreservesTargetChangedAfterPreflight(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	harness := raceHarnessExe(t, cli)
	base := []byte("original game file")
	patched := []byte("Persian replacement")
	concurrent := "changed during patch preparation"
	patchPath := filepath.Join(t.TempDir(), "apply-race.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	target, err := w.AddTarget("Data/file.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256(base)
	if err := target.SetBaseHash(baseHash[:], int64(len(base))); err != nil {
		t.Fatal(err)
	}
	target.AddPayload("Data/file.bin", patched)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	dataDir := filepath.Join(gameDir, "Data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDir, "file.bin")
	if err := os.WriteFile(targetPath, base, 0o644); err != nil {
		t.Fatal(err)
	}
	message := "در حال پشتیبان‌گیری: Data/file.bin"
	if output, err := exec.Command(harness, "apply", patchPath, gameDir, "Data/file.bin", concurrent, message).CombinedOutput(); err == nil {
		t.Fatalf("apply unexpectedly overwrote a concurrent edit: %s", output)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != concurrent {
		t.Fatalf("concurrent target edit was lost: got %q, want %q", got, concurrent)
	}
	for _, suffix := range []string{".ffbak", ".ffnew"} {
		if _, err := os.Stat(targetPath + suffix); !os.IsNotExist(err) {
			t.Errorf("temporary %s remains after rejected apply: %v", suffix, err)
		}
	}
}

func TestPatcherUninstallPreservesTargetChangedAfterPreflight(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	harness := raceHarnessExe(t, cli)
	base := []byte("original game file")
	patched := []byte("Persian replacement")
	concurrent := "changed during uninstall preparation"
	patchPath := filepath.Join(t.TempDir(), "uninstall-race.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	target, err := w.AddTarget("Data/file.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256(base)
	if err := target.SetBaseHash(baseHash[:], int64(len(base))); err != nil {
		t.Fatal(err)
	}
	target.AddPayload("Data/file.bin", patched)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	dataDir := filepath.Join(gameDir, "Data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDir, "file.bin")
	backupPath := targetPath + ".ffbak"
	if err := os.WriteFile(targetPath, patched, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, base, 0o644); err != nil {
		t.Fatal(err)
	}
	message := "در حال بررسی فایل نصب‌شده: Data/file.bin"
	if output, err := exec.Command(harness, "uninstall", patchPath, gameDir, "Data/file.bin", concurrent, message).CombinedOutput(); err == nil {
		t.Fatalf("uninstall unexpectedly overwrote a concurrent edit: %s", output)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != concurrent {
		t.Fatalf("concurrent target edit was lost: got %q, want %q", got, concurrent)
	}
	gotBackup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBackup, base) {
		t.Fatalf("original backup changed: got %q, want %q", gotBackup, base)
	}
	if _, err := os.Stat(targetPath + ".ffnew"); !os.IsNotExist(err) {
		t.Fatalf("temporary patched file remains after rejected uninstall: %v", err)
	}
}

func TestPatcherApplyDoesNotOverwriteTemporaryFileCreatedAfterPreflight(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	harness := raceHarnessExe(t, cli)
	base := []byte("original game file")
	patched := []byte("Persian replacement")
	patchPath := filepath.Join(t.TempDir(), "temp-race.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	target, err := w.AddTarget("Data/file.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256(base)
	if err := target.SetBaseHash(baseHash[:], int64(len(base))); err != nil {
		t.Fatal(err)
	}
	target.AddPayload("Data/file.bin", patched)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	dataDir := filepath.Join(gameDir, "Data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDir, "file.bin")
	if err := os.WriteFile(targetPath, base, 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := "temporary file created after preflight"
	trigger := "در حال اعمال فایل‌ها: Data/file.bin"
	if output, err := exec.Command(harness, "stale-temp", patchPath, gameDir, "Data/file.bin", sentinel, trigger).CombinedOutput(); err == nil {
		t.Fatalf("apply unexpectedly overwrote a racing .ffnew file: %s", output)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, base) {
		t.Fatalf("original target changed: got %q, want %q", got, base)
	}
	gotTemp, err := os.ReadFile(targetPath + ".ffnew")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTemp) != sentinel {
		t.Fatalf("racing temporary file was overwritten or deleted: got %q", gotTemp)
	}
	if _, err := os.Stat(targetPath + ".ffbak"); !os.IsNotExist(err) {
		t.Fatalf("backup created despite failure before swapping: %v", err)
	}
}

func TestPatcherRollbackPreservesConcurrentTargetAndUnownedTemp(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}
	harness := raceHarnessExe(t, cli)
	baseA, patchedA := []byte("original A"), []byte("patched A")
	baseB, patchedB := []byte("original B"), []byte("patched B")
	patchPath := filepath.Join(t.TempDir(), "rollback-race.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	for _, item := range []struct {
		name    string
		base    []byte
		patched []byte
	}{{"a.bin", baseA, patchedA}, {"b.bin", baseB, patchedB}} {
		target, err := w.AddTarget(item.name, ModeReplace)
		if err != nil {
			t.Fatal(err)
		}
		baseHash := sha256.Sum256(item.base)
		if err := target.SetBaseHash(baseHash[:], int64(len(item.base))); err != nil {
			t.Fatal(err)
		}
		target.AddPayload(item.name, item.patched)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{"a.bin": baseA, "b.bin": baseB} {
		if err := os.WriteFile(filepath.Join(gameDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	concurrent := "user edit after first target was installed"
	sentinel := "unowned temporary file"
	if output, err := exec.Command(harness, "apply", patchPath, gameDir,
		"a.bin", concurrent, "نصب شد: a.bin",
		"در حال اعمال فایل‌ها: b.bin", "b.bin", "temp", sentinel).CombinedOutput(); err == nil {
		t.Fatalf("apply unexpectedly succeeded after later target collided: %s", output)
	}
	gotA, err := os.ReadFile(filepath.Join(gameDir, "a.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotA) != concurrent {
		t.Fatalf("concurrent edit to earlier target was lost: got %q", gotA)
	}
	gotBackup, err := os.ReadFile(filepath.Join(gameDir, "a.bin.ffbak"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBackup, baseA) {
		t.Fatalf("original A backup changed: got %q, want %q", gotBackup, baseA)
	}
	gotB, err := os.ReadFile(filepath.Join(gameDir, "b.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotB, baseB) {
		t.Fatalf("B target changed before its swap: got %q", gotB)
	}
	gotTemp, err := os.ReadFile(filepath.Join(gameDir, "b.bin.ffnew"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTemp) != sentinel {
		t.Fatalf("unowned .ffnew was overwritten or deleted: got %q", gotTemp)
	}
	for _, path := range []string{filepath.Join(gameDir, "a.bin.ffnew"), filepath.Join(gameDir, "b.bin.ffbak")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("unexpected temporary/backup %s: %v", path, err)
		}
	}
}

func TestPatcherUninstallPreservesModifiedTargetAndInvalidBackup(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}

	base := []byte("original game file")
	patched := []byte("installed translation")
	modified := []byte("user changed this after installation")
	patchPath := filepath.Join(t.TempDir(), "uninstall-safety.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	target, err := w.AddTarget("Data/file.bin", ModeReplace)
	if err != nil {
		t.Fatal(err)
	}
	baseHash := sha256.Sum256(base)
	if err := target.SetBaseHash(baseHash[:], int64(len(base))); err != nil {
		t.Fatal(err)
	}
	target.AddPayload("Data/file.bin", patched)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	dataDir := filepath.Join(gameDir, "Data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(dataDir, "file.bin")
	backupPath := targetPath + ".ffbak"
	if err := os.WriteFile(targetPath, modified, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupPath, base, 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command(cli, "--uninstall", patchPath, gameDir).CombinedOutput(); err == nil {
		t.Fatalf("uninstall unexpectedly overwrote a modified target: %s", out)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, modified) {
		t.Fatalf("user-modified target was lost: got %q, want %q", got, modified)
	}
	gotBackup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBackup, base) {
		t.Fatalf("original backup changed during rejected uninstall: got %q, want %q", gotBackup, base)
	}

	if err := os.WriteFile(targetPath, patched, 0o644); err != nil {
		t.Fatal(err)
	}
	invalidBackup := []byte("not the original game file")
	if err := os.WriteFile(backupPath, invalidBackup, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cli, "--uninstall", patchPath, gameDir).CombinedOutput(); err == nil {
		t.Fatalf("uninstall accepted a backup that does not match the patch base: %s", out)
	}
	got, err = os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, patched) {
		t.Fatalf("patched target changed after invalid backup was rejected: got %q, want %q", got, patched)
	}
	gotBackup, err = os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBackup, invalidBackup) {
		t.Fatalf("invalid backup was changed after rejection: got %q, want %q", gotBackup, invalidBackup)
	}
}

func TestPatcherUninstallPreflightsAllTargetsBeforeRestoring(t *testing.T) {
	cli := cliExe()
	if cli == "" {
		t.Skip("FarsiForgePatcherCli.exe not built; run patcher/build.ps1 first")
	}

	type filePair struct {
		base     []byte
		patched  []byte
		modified []byte
	}
	files := []struct {
		name string
		filePair
	}{
		{name: "a.bin", filePair: filePair{base: []byte("original a"), patched: []byte("patched a")}},
		{name: "b.bin", filePair: filePair{base: []byte("original b"), patched: []byte("patched b"), modified: []byte("user changed b")}},
	}
	patchPath := filepath.Join(t.TempDir(), "uninstall-preflight.ffpatch")
	patchFile, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWriter(patchFile)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{GameName: "TestGame", GameExe: "TestGame.exe"})
	for _, file := range files {
		target, err := w.AddTarget(file.name, ModeReplace)
		if err != nil {
			t.Fatal(err)
		}
		baseHash := sha256.Sum256(file.base)
		if err := target.SetBaseHash(baseHash[:], int64(len(file.base))); err != nil {
			t.Fatal(err)
		}
		target.AddPayload(file.name, file.patched)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := patchFile.Close(); err != nil {
		t.Fatal(err)
	}

	gameDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(gameDir, "TestGame.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		current := file.patched
		if file.modified != nil {
			current = file.modified
		}
		if err := os.WriteFile(filepath.Join(gameDir, file.name), current, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gameDir, file.name+".ffbak"), file.base, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command(cli, "--uninstall", patchPath, gameDir).CombinedOutput(); err == nil {
		t.Fatalf("uninstall unexpectedly succeeded with a modified second target: %s", out)
	}
	for _, file := range files {
		want := file.patched
		if file.modified != nil {
			want = file.modified
		}
		got, err := os.ReadFile(filepath.Join(gameDir, file.name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s changed despite preflight failure: got %q, want %q", file.name, got, want)
		}
		gotBackup, err := os.ReadFile(filepath.Join(gameDir, file.name+".ffbak"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gotBackup, file.base) {
			t.Errorf("%s backup changed despite preflight failure: got %q, want %q", file.name, gotBackup, file.base)
		}
	}
}
