package inject

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"farsiforge/pkg/core"
	"farsiforge/pkg/installer"
	"farsiforge/pkg/tools"
)

// TestGodotInjectorPCKIntegration is opt-in and should use a disposable PCK.
// It verifies translation import, PCK rebuilding, staging, and extraction of
// the exact updated message without touching an installed game.
func TestGodotInjectorPCKIntegration(t *testing.T) {
	pckPath := os.Getenv("FF_GODOT_INJECT_PCK")
	fftools := os.Getenv("FFTOOLS_EXE")
	if pckPath == "" || fftools == "" {
		t.Skip("set FF_GODOT_INJECT_PCK and FFTOOLS_EXE to a disposable PCK and built fftools executable")
	}
	pckPath, err := filepath.Abs(pckPath)
	if err != nil {
		t.Fatal(err)
	}
	container := filepath.Base(pckPath)
	gameRoot := filepath.Dir(pckPath)
	registry := &tools.Registry{FFTools: fftools, Available: map[string]bool{"fftools": true}}
	workDir := t.TempDir()
	project := core.NewProject("Godot injector integration", gameRoot, "godot")
	project.WorkingDir = workDir

	inputBytes, err := os.ReadFile(pckPath)
	if err != nil {
		t.Fatal(err)
	}
	inputExtract := filepath.Join(workDir, "input")
	if out, err := tools.RunSilent(context.Background(), workDir, fftools, "pck", "extract", pckPath, "-o", inputExtract); err != nil {
		t.Fatalf("extract integration PCK: %v\n%s", err, out)
	}
	var resourcePath, catalogPath string
	for _, file := range []string{
		filepath.Join(inputExtract, "locale", "fa.translation"),
	} {
		if fileExists(file) {
			resourcePath = file
			catalogPath = findGodotSourceCatalog(file, inputExtract)
			break
		}
	}
	if resourcePath == "" || catalogPath == "" {
		t.Fatal("disposable PCK must contain locale/fa.translation and its matching CSV")
	}
	sourceLanguage, err := godotSourceLanguage(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	exportedPath := filepath.Join(workDir, "messages.json")
	if out, err := tools.RunSilent(context.Background(), workDir, fftools, "translation", "export", resourcePath, catalogPath, "--source-language", sourceLanguage, "-o", exportedPath); err != nil {
		t.Fatalf("export source messages: %v\n%s", err, out)
	}
	encoded, err := os.ReadFile(exportedPath)
	if err != nil {
		t.Fatal(err)
	}
	var messages []godotTranslationMessage
	if err := json.Unmarshal(encoded, &messages); err != nil || len(messages) == 0 {
		t.Fatalf("parse exported messages: count=%d err=%v", len(messages), err)
	}
	message := messages[0]
	relResource, err := filepath.Rel(inputExtract, resourcePath)
	if err != nil {
		t.Fatal(err)
	}
	entry := core.StringEntry{
		ID:          "godot:" + container + "::" + filepath.ToSlash(relResource) + "::" + message.ID,
		Source:      message.Source,
		Translation: "تزریق آزمایشی",
		File:        filepath.ToSlash(relResource),
		Container:   container,
		Path:        message.Key,
		Context:     "Godot translation " + message.Locale + " (source " + sourceLanguage + ")",
		Status:      core.StatusTranslated,
	}
	project.AddEntry(entry)

	result, err := (&GodotInjector{}).Inject(context.Background(), &core.GameInfo{Engine: "godot", GameRoot: gameRoot}, project, registry, core.PersianOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ModifiedFiles) != 1 || result.ModifiedFiles[0] != container || result.StringCount != 1 {
		t.Fatalf("injection result = %+v", result)
	}
	patchedPCK := filepath.Join(workDir, "out", container)
	if !fileExists(patchedPCK) {
		t.Fatalf("staged PCK missing: %s", patchedPCK)
	}
	if engineCheckPath := os.Getenv("FF_GODOT_INJECT_OUT"); engineCheckPath != "" {
		if err := copyStagedFile(patchedPCK, engineCheckPath); err != nil {
			t.Fatalf("preserve staged PCK for engine check: %v", err)
		}
	}
	patchedExtract := filepath.Join(workDir, "patched")
	if out, err := tools.RunSilent(context.Background(), workDir, fftools, "pck", "extract", patchedPCK, "-o", patchedExtract); err != nil {
		t.Fatalf("extract staged PCK: %v\n%s", err, out)
	}
	resourcePath = filepath.Join(patchedExtract, filepath.FromSlash(filepath.ToSlash(relResource)))
	exportedPath = filepath.Join(workDir, "verify.json")
	if out, err := tools.RunSilent(context.Background(), workDir, fftools, "translation", "export", resourcePath, catalogPath, "--source-language", sourceLanguage, "-o", exportedPath); err != nil {
		t.Fatalf("export staged translation: %v\n%s", err, out)
	}
	encoded, err = os.ReadFile(exportedPath)
	if err != nil {
		t.Fatal(err)
	}
	messages = nil
	if err := json.Unmarshal(encoded, &messages); err != nil {
		t.Fatal(err)
	}
	for _, got := range messages {
		if got.ID == message.ID {
			if got.Translation != "تزریق آزمایشی" {
				t.Fatalf("rebuilt PCK translation = %q", got.Translation)
			}
			unchanged, err := os.ReadFile(pckPath)
			if err != nil || string(unchanged) != string(inputBytes) {
				t.Fatalf("source PCK was modified: read err=%v", err)
			}
			verifyGodotPatchInstallRoundTrip(t, workDir, gameRoot, container, pckPath, patchedPCK)
			return
		}
	}
	t.Fatalf("message %q was missing from staged resource; exported IDs: %s", message.ID, strings.Join(godotMessageIDs(messages), ", "))
}

func verifyGodotPatchInstallRoundTrip(t *testing.T, workDir, gameRoot, container, originalPCK, patchedPCK string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	patcherCLI := filepath.Join(root, "Tools", "patcher", "FarsiForgePatcherCli.exe")
	patcherGUI := filepath.Join(root, "Tools", "patcher", "FarsiForgePatcher.exe")
	for _, path := range []string{patcherCLI, patcherGUI} {
		if !fileExists(path) {
			t.Skipf("built patcher executable missing: %s", path)
		}
	}
	installGame := filepath.Join(workDir, "install-game")
	if err := os.MkdirAll(installGame, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyStagedFile(originalPCK, filepath.Join(installGame, filepath.FromSlash(container))); err != nil {
		t.Fatalf("copy original PCK into disposable game: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installGame, "TestGame.exe"), []byte("disposable game marker"), 0o644); err != nil {
		t.Fatal(err)
	}

	packageDir := filepath.Join(workDir, "installer-package")
	built, err := installer.Build(installer.BuildConfig{
		GameRoot:   gameRoot,
		Targets:    []installer.PatchTarget{{GamePath: container, PatchedFile: patchedPCK, OriginalFile: originalPCK}},
		PatcherExe: patcherGUI,
		GameExe:    "TestGame.exe",
		Engine:     "Godot",
		PatchName:  "Godot integration patch",
		OutputDir:  packageDir,
	})
	if err != nil {
		t.Fatalf("build FFP installer package: %v", err)
	}
	if built.TargetCount != 1 || !fileExists(built.PatcherExe) {
		t.Fatalf("unexpected installer build result: %+v", built)
	}
	apply := exec.Command(patcherCLI, built.PatchFile, installGame)
	if output, err := apply.CombinedOutput(); err != nil {
		t.Fatalf("apply FFP package: %v\n%s", err, output)
	}
	installed, err := os.ReadFile(filepath.Join(installGame, filepath.FromSlash(container)))
	if err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(patchedPCK)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != string(patched) {
		t.Fatal("FFP installation did not install the rebuilt Godot PCK")
	}
	uninstall := exec.Command(patcherCLI, "--uninstall", built.PatchFile, installGame)
	if output, err := uninstall.CombinedOutput(); err != nil {
		t.Fatalf("uninstall FFP package: %v\n%s", err, output)
	}
	restored, err := os.ReadFile(filepath.Join(installGame, filepath.FromSlash(container)))
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(originalPCK)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatal("FFP uninstall did not restore the original Godot PCK byte-for-byte")
	}
}

func godotMessageIDs(messages []godotTranslationMessage) []string {
	ids := make([]string, len(messages))
	for i, message := range messages {
		ids[i] = message.ID
	}
	return ids
}
