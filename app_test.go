package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"farsiforge/pkg/core"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/tools"
)

func TestExtractStoresProjectOutsideGameDirectory(t *testing.T) {
	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(core.EnvProjectsDir, store)
	if err := os.WriteFile(filepath.Join(gameRoot, "locale.ini"), []byte("title=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	count, err := app.Extract(&core.GameInfo{GameRoot: gameRoot, GameName: "Sample", Engine: "generic"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("extracted entries = %d, want 1", count)
	}
	projectPath, err := core.ProjectFilePath(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(projectPath); err != nil {
		t.Fatalf("external project file missing: %v", err)
	}
	project, err := core.LoadProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if project.GameName != "Sample" {
		t.Fatalf("project game name = %q, want Sample", project.GameName)
	}
	for _, legacy := range []string{core.LegacyProjectFileName, "work"} {
		if _, err := os.Stat(filepath.Join(gameRoot, legacy)); !os.IsNotExist(err) {
			t.Fatalf("unexpected game-directory artifact %q (stat error %v)", legacy, err)
		}
	}
}

func TestBuildPatcherUsesStagedFilesAndExternalOutput(t *testing.T) {
	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(core.EnvProjectsDir, store)
	project, projectPath, err := core.NewGameProject("Sample Localization", gameRoot, "generic")
	if err != nil {
		t.Fatal(err)
	}
	project.GameName = "Sample"
	project.ModifiedFiles = []string{"Sample_Data/locale.ini"}
	originalPath := filepath.Join(gameRoot, "Sample_Data", "locale.ini")
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(originalPath, []byte("title=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256([]byte("title=Hello\n"))
	project.ModifiedFileHashes = map[string]string{"Sample_Data/locale.ini": hex.EncodeToString(sourceHash[:])}
	patchedPath := filepath.Join(project.WorkingDir, "out", "Sample_Data", "locale.ini")
	if err := os.MkdirAll(filepath.Dir(patchedPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patchedPath, []byte("title=خوش آمدید\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := project.Save(projectPath); err != nil {
		t.Fatal(err)
	}
	toolsDir := t.TempDir()
	patcherPath := filepath.Join(toolsDir, "patcher", "FarsiForgePatcher.exe")
	if err := os.MkdirAll(filepath.Dir(patcherPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patcherPath, []byte("test patcher"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{toolReg: &tools.Registry{RootDir: toolsDir}}
	result, err := app.BuildPatcher(gameRoot, "Translator")
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetCount != 1 {
		t.Fatalf("target count = %d, want 1", result.TargetCount)
	}
	if _, err := os.Stat(result.PatchFile); err != nil {
		t.Fatalf("patch file missing: %v", err)
	}
	patchData, err := os.ReadFile(result.PatchFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(patchData) < 8 || string(patchData[:4]) != "FFP1" || binary.LittleEndian.Uint32(patchData[4:8]) != 2 {
		t.Fatal("installer should build an FFP1 v2 patch with original-file verification")
	}
	rel, err := filepath.Rel(gameRoot, result.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "." || (rel != ".." && len(rel) >= 2 && rel[:2] != "..") {
		t.Fatalf("patch output is inside game directory: %s", result.OutputDir)
	}
}

func TestReextractPreservesTranslationsByFullEntryIdentity(t *testing.T) {
	gameRoot := t.TempDir()
	t.Setenv(core.EnvProjectsDir, t.TempDir())
	localePath := filepath.Join(gameRoot, "locale.ini")
	if err := os.WriteFile(localePath, []byte("title=Welcome\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := &App{}
	info := &core.GameInfo{GameRoot: gameRoot, GameName: "Sample", Engine: "generic"}
	if _, err := app.Extract(info); err != nil {
		t.Fatal(err)
	}
	project, err := app.GetProject(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	project.Entries[0].Translation = "خوش آمدید"
	project.Entries[0].Status = core.StatusApproved
	if err := app.SaveTranslations(gameRoot, project.Entries); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localePath, []byte("title=Welcome\nhelp=Read the journal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Extract(info); err != nil {
		t.Fatal(err)
	}
	project, err = app.GetProject(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Entries) != 2 {
		t.Fatalf("re-extraction entries = %d, want 2", len(project.Entries))
	}
	if project.Entries[0].Translation != "خوش آمدید" || project.Entries[0].Status != core.StatusApproved {
		t.Fatalf("existing translation was not preserved: %+v", project.Entries[0])
	}
}

func TestSaveTranslationsRejectsStaleAndDuplicateIDsWithoutChangingProject(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []core.StringEntry
	}{
		{
			name: "stale ID",
			entries: []core.StringEntry{
				{ID: "entry-1", Translation: "تغییر نشده", Status: core.StatusTranslated},
				{ID: "removed-entry", Translation: "قدیمی", Status: core.StatusTranslated},
			},
		},
		{
			name: "duplicate ID",
			entries: []core.StringEntry{
				{ID: "entry-1", Translation: "اول", Status: core.StatusTranslated},
				{ID: "entry-1", Translation: "دوم", Status: core.StatusTranslated},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gameRoot := t.TempDir()
			t.Setenv(core.EnvProjectsDir, t.TempDir())
			project, projectPath, err := core.NewGameProject("Sample", gameRoot, "generic")
			if err != nil {
				t.Fatal(err)
			}
			project.AddEntry(core.StringEntry{
				ID: "entry-1", File: "locale.ini", Path: "title", Source: "Hello",
				Translation: "سلام", Status: core.StatusApproved,
			})
			if err := project.Save(projectPath); err != nil {
				t.Fatal(err)
			}
			if err := (&App{}).SaveTranslations(gameRoot, tc.entries); err == nil {
				t.Fatal("SaveTranslations accepted ambiguous or stale entry IDs")
			}
			saved, err := core.LoadGameProject(gameRoot)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Entries[0].Translation != "سلام" || saved.Entries[0].Status != core.StatusApproved {
				t.Fatalf("invalid save changed project entry: %+v", saved.Entries[0])
			}
		})
	}
}

func TestEndToEndTranslateBuildApplyAndUninstall(t *testing.T) {
	cli := filepath.Join("patcher", "bin", "FarsiForgePatcherCli.exe")
	if _, err := os.Stat(cli); err != nil {
		cli = filepath.Join("Tools", "patcher", "FarsiForgePatcherCli.exe")
	}
	if _, err := os.Stat(cli); err != nil {
		t.Skip("build the C# patcher first with patcher/build.ps1")
	}

	gameRoot := t.TempDir()
	store := t.TempDir()
	t.Setenv(core.EnvProjectsDir, store)
	original := []byte("title=Hello\n")
	localePath := filepath.Join(gameRoot, "locale.ini")
	if err := os.WriteFile(localePath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameRoot, "sample.exe"), []byte("exe"), 0o644); err != nil {
		t.Fatal(err)
	}
	toolsDir := t.TempDir()
	patcherExe := filepath.Join(toolsDir, "patcher", "FarsiForgePatcher.exe")
	if err := os.MkdirAll(filepath.Dir(patcherExe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patcherExe, []byte("gui patcher placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	info := &core.GameInfo{GameRoot: gameRoot, GameName: "Sample", GameExe: "sample.exe", Engine: "generic"}
	app := &App{toolReg: &tools.Registry{RootDir: toolsDir}}
	if _, err := app.Extract(info); err != nil {
		t.Fatal(err)
	}
	project, err := app.GetProject(gameRoot)
	if err != nil {
		t.Fatal(err)
	}
	project.Entries[0].Translation = "سلام"
	project.Entries[0].Status = core.StatusApproved
	if err := app.SaveTranslations(gameRoot, project.Entries); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Inject(info); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(localePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatal("translation staging modified the installed game file")
	}
	packageResult, err := app.BuildPatcher(gameRoot, "Test Translator")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cli, packageResult.PatchFile, gameRoot).CombinedOutput(); err != nil {
		t.Fatalf("patch apply failed: %v\n%s", err, out)
	}
	installed, err := os.ReadFile(localePath)
	if err != nil {
		t.Fatal(err)
	}
	wantInstalled := "title=" + persian.Process("سلام", persian.Options{
		Reshape: true, BidiReorder: true, FixYeh: true, PersianDigits: true,
	}) + "\n"
	if string(installed) != wantInstalled {
		t.Fatalf("installed translation = %q, want %q", installed, wantInstalled)
	}
	backup, err := os.ReadFile(localePath + ".ffbak")
	if err != nil || string(backup) != string(original) {
		t.Fatalf("original backup = %q, err=%v", backup, err)
	}
	if out, err := exec.Command(cli, "--uninstall", packageResult.PatchFile, gameRoot).CombinedOutput(); err != nil {
		t.Fatalf("patch uninstall failed: %v\n%s", err, out)
	}
	restored, err := os.ReadFile(localePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("uninstall restored %q, want %q", restored, original)
	}
}
