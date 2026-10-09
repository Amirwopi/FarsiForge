package extract

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farsiforge/pkg/core"
	"farsiforge/pkg/tools"
)

func TestFindGodotCatalogMatchesLocaleSuffixedResource(t *testing.T) {
	dir := t.TempDir()
	translation := filepath.Join(dir, "Game Content.FA.translation")
	catalog := filepath.Join(dir, "Game Content.csv")
	other := filepath.Join(dir, "Other.csv")
	got := findGodotCatalog(translation, []string{other, catalog})
	if got != catalog {
		t.Fatalf("findGodotCatalog() = %q, want %q", got, catalog)
	}
}

func TestFindGodotCatalogRejectsAmbiguousFallback(t *testing.T) {
	dir := t.TempDir()
	translation := filepath.Join(dir, "unmatched.FA.translation")
	one := filepath.Join(dir, "one.csv")
	two := filepath.Join(dir, "two.csv")
	if got := findGodotCatalog(translation, []string{one, two}); got != "" {
		t.Fatalf("findGodotCatalog() = %q, want no ambiguous match", got)
	}
}

func TestGodotCSVSourceLanguagePrefersEnglishAndHandlesBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.csv")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbfkey,FR,EN,FA\nstart,Démarrer,Start,شروع\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := godotCSVSourceLanguage(path)
	if err != nil || got != "EN" {
		t.Fatalf("godotCSVSourceLanguage() = %q, %v", got, err)
	}
}

func TestGodotCSVSourceLanguageFallsBackToFirstLanguageColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.csv")
	if err := os.WriteFile(path, []byte("key,FR,FA\nstart,Démarrer,شروع\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := godotCSVSourceLanguage(path)
	if err != nil || got != "FR" {
		t.Fatalf("godotCSVSourceLanguage() = %q, %v", got, err)
	}
}

// TestGodotTranslationExtractionIntegration is opt-in because it needs a
// local Godot translation catalog and a built fftools executable.
func TestGodotTranslationExtractionIntegration(t *testing.T) {
	dir := os.Getenv("FF_GODOT_TRANSLATION_DIR")
	fftools := os.Getenv("FFTOOLS_EXE")
	if dir == "" || fftools == "" {
		t.Skip("set FF_GODOT_TRANSLATION_DIR and FFTOOLS_EXE to opt in")
	}
	project := &core.Project{}
	registry := &tools.Registry{FFTools: fftools}
	workDir := t.TempDir()
	(&GodotExtractor{}).exportTranslationFiles(context.Background(), project, registry, workDir, dir, "sample/game.pck")
	if len(project.Entries) < 269 {
		t.Fatalf("extracted %d translation entries, want at least one 269-entry locale", len(project.Entries))
	}
	ids := make(map[string]bool, len(project.Entries))
	files := make(map[string]bool)
	locales := make(map[string]bool)
	translated := 0
	for _, entry := range project.Entries {
		if entry.ID == "" || entry.Path == "" || entry.File == "" || entry.Source == "" || entry.Container != "sample/game.pck" {
			t.Errorf("incomplete Godot translation entry: %#v", entry)
		}
		if ids[entry.ID] {
			t.Errorf("duplicate Godot translation ID %q", entry.ID)
		}
		ids[entry.ID] = true
		files[entry.File] = true
		if start := strings.Index(entry.Context, "Godot translation "); start >= 0 {
			locale := strings.TrimPrefix(entry.Context[start:], "Godot translation ")
			if end := strings.Index(locale, " "); end >= 0 {
				locales[locale[:end]] = true
			}
		}
		if entry.Status == core.StatusTranslated {
			translated++
		}
	}
	if translated == 0 {
		t.Fatal("all Godot translation entries were marked untranslated")
	}
	t.Logf("mapped entries=%d resources=%d locales=%d translated=%d", len(project.Entries), len(files), len(locales), translated)
}
