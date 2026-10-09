package core

import "testing"

func TestMergeTranslationsDistinguishesContainers(t *testing.T) {
	previous := &Project{Entries: []StringEntry{{
		ID:          "same-entry-id",
		Container:   "base.pck",
		File:        "locale/fa.translation",
		Path:        "MENU_START",
		Context:     "Godot translation fa",
		Source:      "Start",
		Translation: "شروع",
		Status:      StatusApproved,
	}}}
	current := &Project{Entries: []StringEntry{
		{ID: "same-entry-id", Container: "base.pck", File: "locale/fa.translation", Path: "MENU_START", Context: "Godot translation fa", Source: "Start", Status: StatusUntranslated},
		{ID: "same-entry-id", Container: "dlc.pck", File: "locale/fa.translation", Path: "MENU_START", Context: "Godot translation fa", Source: "Start", Status: StatusUntranslated},
	}}
	if merged := current.MergeTranslations(previous); merged != 1 {
		t.Fatalf("MergeTranslations() = %d, want 1", merged)
	}
	if current.Entries[0].Translation != "شروع" || current.Entries[0].Status != StatusApproved {
		t.Fatalf("matching container did not preserve translation: %+v", current.Entries[0])
	}
	if current.Entries[1].Translation != "" || current.Entries[1].Status != StatusUntranslated {
		t.Fatalf("translation crossed container boundary: %+v", current.Entries[1])
	}
}

func TestMergeTranslationsDoesNotUseDelimiterBasedIdentity(t *testing.T) {
	previous := &Project{Entries: []StringEntry{{
		ID:          "old",
		File:        "a\x00b",
		Path:        "c",
		Context:     "d",
		Source:      "e",
		Translation: "قدیمی",
		Status:      StatusApproved,
	}}}
	current := &Project{Entries: []StringEntry{{
		ID:      "new",
		File:    "a",
		Path:    "b\x00c",
		Context: "d",
		Source:  "e",
		Status:  StatusUntranslated,
	}}}
	if merged := current.MergeTranslations(previous); merged != 0 {
		t.Fatalf("MergeTranslations() = %d, want 0 for distinct field tuples", merged)
	}
	if current.Entries[0].Translation != "" || current.Entries[0].Status != StatusUntranslated {
		t.Fatalf("translation crossed identity fields: %+v", current.Entries[0])
	}
}
