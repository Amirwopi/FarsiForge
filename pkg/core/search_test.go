package core

import "testing"

func TestSearchEntries(t *testing.T) {
	p := NewProject("test", "/tmp/game", "unreal")
	p.AddEntry(StringEntry{ID: "s_0001", Source: "You found {0} items", File: "Game.uexp", Status: StatusUntranslated})
	p.AddEntry(StringEntry{ID: "s_0002", Source: "Press fire", Translation: "شلیک کنید", File: "Game.uexp", Status: StatusTranslated, Notes: "MISMATCH_TAGS: missing <cf>"})
	p.AddEntry(StringEntry{ID: "s_0003", Source: "Hello, world!", File: "Menu.locres", Status: StatusApproved})

	// Query matches source (case-insensitive).
	if got := len(p.SearchEntries("FOUND", "")); got != 1 {
		t.Errorf("query 'FOUND': expected 1 match, got %d", got)
	}
	// Query matches translation (Persian).
	if got := len(p.SearchEntries("شلیک", "")); got != 1 {
		t.Errorf("Persian query: expected 1 match, got %d", got)
	}
	// Query matches file name.
	if got := len(p.SearchEntries("menu.locres", "")); got != 1 {
		t.Errorf("file query: expected 1 match, got %d", got)
	}
	// Query matches notes.
	if got := len(p.SearchEntries("MISMATCH_TAGS", "")); got != 1 {
		t.Errorf("notes query: expected 1 match, got %d", got)
	}
	// Query matches ID.
	if got := len(p.SearchEntries("s_0003", "")); got != 1 {
		t.Errorf("ID query: expected 1 match, got %d", got)
	}
	// Status filter alone.
	if got := len(p.SearchEntries("", "approved")); got != 1 {
		t.Errorf("status filter: expected 1 match, got %d", got)
	}
	// QA pseudo-status (entries with notes).
	if got := len(p.SearchEntries("", "qa")); got != 1 {
		t.Errorf("qa filter: expected 1 match, got %d", got)
	}
	// Combined query + status.
	if got := len(p.SearchEntries("press", "translated")); got != 1 {
		t.Errorf("combined query+status: expected 1 match, got %d", got)
	}
	if got := len(p.SearchEntries("press", "untranslated")); got != 0 {
		t.Errorf("combined query+status: expected 0 matches, got %d", got)
	}
	// Empty query + empty status returns all entries.
	if got := len(p.SearchEntries("", "")); got != 3 {
		t.Errorf("no filter: expected 3 matches, got %d", got)
	}
	// No match.
	if got := len(p.SearchEntries("nonexistent", "")); got != 0 {
		t.Errorf("no-match query: expected 0 matches, got %d", got)
	}
}
