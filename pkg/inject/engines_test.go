package inject

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farsiforge/pkg/core"
)

func TestGodotInjectorRequiresFFTools(t *testing.T) {
	injector := &GodotInjector{}
	caps := injector.Capabilities()
	if !caps.TextInjection || !caps.NeedsExternalTool || caps.ToolName != "fftools" {
		t.Fatalf("Godot capabilities = %+v, want translation injection through fftools", caps)
	}
	result, err := injector.Inject(context.Background(), &core.GameInfo{}, &core.Project{}, nil, core.PersianOptions{})
	if err == nil || result != nil {
		t.Fatalf("Inject() = (%v, %v), want a missing-tool error", result, err)
	}
}

func TestGodotPersianLocale(t *testing.T) {
	for _, locale := range []string{"fa", "FA", "fa_IR", "fa-IR", "fa-IR-u-nu-latn"} {
		if !isPersianGodotLocale(locale) {
			t.Errorf("isPersianGodotLocale(%q) = false", locale)
		}
	}
	for _, locale := range []string{"en", "fr", "far", ""} {
		if isPersianGodotLocale(locale) {
			t.Errorf("isPersianGodotLocale(%q) = true", locale)
		}
	}
}

func TestUnityInjectorRejectsRawScanEntriesBeforeRunningTools(t *testing.T) {
	project := core.NewProject("raw Unity candidate", t.TempDir(), "unity")
	project.AddEntry(core.StringEntry{
		ID: "asset_42_raw_0", File: "Game_Data/resources.assets", Path: "raw_0",
		Context: "MonoBehaviour", Source: "Visible dialogue", Translation: "دیالوگ",
		Status: core.StatusTranslated,
	})

	result, err := (&UnityInjector{}).Inject(context.Background(), &core.GameInfo{GameRoot: project.GameRoot}, project, nil, core.PersianOptions{})
	if err == nil || result == nil {
		t.Fatalf("Inject() = (%v, %v), want explicit rejection before tool access", result, err)
	}
	if !strings.Contains(err.Error(), "raw-byte scan") || !strings.Contains(err.Error(), "typetree field identity") {
		t.Fatalf("Inject() error = %q, want raw-scan and missing-field explanation", err)
	}
}

func TestFindGodotSourceCatalogRequiresUnambiguousSibling(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "locale")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	translation := filepath.Join(dir, "Game.FA.translation")
	catalog := filepath.Join(dir, "Game.csv")
	other := filepath.Join(dir, "Other.csv")
	for _, file := range []string{translation, catalog, other} {
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := findGodotSourceCatalog(translation, root); got != catalog {
		t.Fatalf("findGodotSourceCatalog() = %q, want %q", got, catalog)
	}
	if got := findGodotSourceCatalog(filepath.Join(dir, "Unmatched.FA.translation"), root); got != "" {
		t.Fatalf("ambiguous fallback catalog = %q, want empty", got)
	}
}

func TestGenericInjectorWritesOnlyStagedFiles(t *testing.T) {
	gameRoot := t.TempDir()
	workDir := t.TempDir()
	sourcePath := filepath.Join(gameRoot, "locale", "ui.ini")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("start = Welcome\r\nkeep = unchanged\r\n")
	if err := os.WriteFile(sourcePath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	project := core.NewProject("test", gameRoot, "generic")
	project.WorkingDir = workDir
	project.ExtractedFiles = []string{filepath.ToSlash(filepath.Join("locale", "ui.ini"))}
	project.Entries = []core.StringEntry{{
		ID:          "entry-1",
		Source:      "Welcome",
		Translation: "خوش آمدید",
		File:        filepath.ToSlash(filepath.Join("locale", "ui.ini")),
		Path:        "start",
		Status:      core.StatusTranslated,
	}}

	result, err := (&GenericInjector{}).Inject(context.Background(), &core.GameInfo{GameRoot: gameRoot}, project, nil, core.PersianOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ModifiedFiles) != 1 || result.ModifiedFiles[0] != "locale/ui.ini" {
		t.Fatalf("modified files = %v", result.ModifiedFiles)
	}
	gotOriginal, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotOriginal) != string(original) {
		t.Fatalf("source game file was modified: %q", gotOriginal)
	}
	patchedPath := filepath.Join(workDir, "out", "locale", "ui.ini")
	gotPatched, err := os.ReadFile(patchedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotPatched) != "start = خوش آمدید\r\nkeep = unchanged\r\n" {
		t.Fatalf("staged output = %q", gotPatched)
	}
}

func TestInjectIntoTextFileRejectsMissingTranslatedKey(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.ini")
	output := filepath.Join(root, "staged", "source.ini")
	if err := os.WriteFile(source, []byte("present = value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := core.StringEntry{File: "source.ini", Path: "missing", Translation: "ترجمه"}
	if _, err := injectIntoTextFile(source, output, "source.ini", []core.StringEntry{entry}, core.PersianOptions{}); err == nil {
		t.Fatal("expected missing translated key to fail")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("staged output should not exist after a missing key: stat err=%v", err)
	}
}

func TestInjectIntoTextFileRejectsDuplicateTranslatedKey(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.ini")
	output := filepath.Join(root, "staged", "source.ini")
	if err := os.WriteFile(source, []byte("label = first\nlabel = second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := core.StringEntry{File: "source.ini", Path: "label", Translation: "ترجمه"}
	if _, err := injectIntoTextFile(source, output, "source.ini", []core.StringEntry{entry}, core.PersianOptions{}); err == nil {
		t.Fatal("expected duplicate translated key to fail")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("staged output should not exist after a duplicate key: stat err=%v", err)
	}
}
