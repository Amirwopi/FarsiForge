package core

import (
	"strings"
	"testing"
)

// newTestProject returns a project with a few entries for QA-notes tests.
func newTestProject() *Project {
	p := NewProject("test", "/tmp/game", "unreal")
	p.AddEntry(StringEntry{ID: "s_0001", Source: "You found {0} items", File: "Game.uexp"})
	p.AddEntry(StringEntry{ID: "s_0002", Source: "Press <cf>fire</cf> to attack", File: "Game.uexp"})
	p.AddEntry(StringEntry{ID: "s_0003", Source: "Hello, world!", File: "Game.uexp"})
	return p
}

func TestSetTranslationQANotes(t *testing.T) {
	p := newTestProject()

	// Missing placeholder → MISMATCH_PLACEHOLDERS note.
	if err := p.SetTranslation("s_0001", "شما آیتم پیدا کردید", StatusTranslated); err != nil {
		t.Fatalf("SetTranslation: %v", err)
	}
	if got := p.Entries[0].Notes; !strings.Contains(got, "MISMATCH_PLACEHOLDERS") {
		t.Errorf("expected MISMATCH_PLACEHOLDERS in Notes, got %q", got)
	}

	// Correct translation → no notes.
	if err := p.SetTranslation("s_0001", "شما {0} آیتم پیدا کردید", StatusTranslated); err != nil {
		t.Fatalf("SetTranslation: %v", err)
	}
	if got := p.Entries[0].Notes; got != "" {
		t.Errorf("expected empty Notes for correct translation, got %q", got)
	}

	// Tag mismatch → MISMATCH_TAGS note.
	if err := p.SetTranslation("s_0002", "برای حمله <cf>شلیک</x> را بزنید", StatusTranslated); err != nil {
		t.Fatalf("SetTranslation: %v", err)
	}
	if got := p.Entries[1].Notes; !strings.Contains(got, "MISMATCH_TAGS") {
		t.Errorf("expected MISMATCH_TAGS in Notes, got %q", got)
	}

	// Empty translation → EMPTY_TRANSLATION note (stale notes are replaced).
	if err := p.SetTranslation("s_0002", "", StatusUntranslated); err != nil {
		t.Fatalf("SetTranslation: %v", err)
	}
	if got := p.Entries[1].Notes; !strings.Contains(got, "EMPTY_TRANSLATION") {
		t.Errorf("expected EMPTY_TRANSLATION in Notes, got %q", got)
	}

	// Unknown ID → error.
	if err := p.SetTranslation("s_9999", "x", StatusTranslated); err == nil {
		t.Error("expected error for unknown ID")
	}
}

func TestImportTranslationsQANotes(t *testing.T) {
	p := newTestProject()

	count := p.ImportTranslations(map[string]string{
		"You found {0} items":           "شما {0} آیتم پیدا کردید", // correct
		"Press <cf>fire</cf> to attack": "برای حمله شلیک کنید",     // missing tags
		"Hello, world!":                 "سلام دنیا!",              // correct
	})
	if count != 3 {
		t.Fatalf("expected 3 merged entries, got %d", count)
	}

	if got := p.Entries[0].Notes; got != "" {
		t.Errorf("expected empty Notes for correct translation, got %q", got)
	}
	if got := p.Entries[1].Notes; !strings.Contains(got, "MISMATCH_TAGS") {
		t.Errorf("expected MISMATCH_TAGS in Notes, got %q", got)
	}
	if got := p.Entries[2].Notes; got != "" {
		t.Errorf("expected empty Notes for correct translation, got %q", got)
	}

	// Empty translations in the map are skipped entirely (no status/notes change).
	p2 := newTestProject()
	if n := p2.ImportTranslations(map[string]string{"Hello, world!": ""}); n != 0 {
		t.Errorf("expected 0 merged entries for empty translation, got %d", n)
	}
	if got := p2.Entries[2].Notes; got != "" {
		t.Errorf("expected untouched Notes for skipped entry, got %q", got)
	}
}
