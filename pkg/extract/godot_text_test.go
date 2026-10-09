package extract

import (
	"strings"
	"testing"
)

func TestIsGodotText(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		// Real text — keep
		{"You should visit Dr. Lee. She is a good soul.", true},
		{"GodotSteam GDExtension updater functionality enabled", true},
		{"Install", true},
		{"Cancel", true},
		{"Up-to-date", true},
		{"A~~ You're hurting me on purpose, right?", true},
		// Paths / uids / bbcode — skip
		{"res://addons/sprouty_dialogs/resources/dialogue_data.gd", false},
		{"uid://cbjdca3iwlx36", false},
		{"[url=https://godotsteam.com]website[/url]", false},
		{"steam/updates/godotsteam/check_for_updates", false},
		// Version chrome — no translatable word ("GodotSteam" camelCase,
		// "v" single letter, the rest is %s placeholders and pipes).
		{"GodotSteam v%s | %s | %s", false},
		// Keys / identifiers — skip
		{"DEMO_DIALOG_54", false},
		{"dialogue_node_54", false},
		{"options_node_15", false},
		{"DEMO_OPT15_1", false},
		{"dialog_char_sister", false},
		{"npc.sister.name", false},
		{"GodotSteam", false},
		{"Vector2", false},
		{"check_for_updates", false},
		{"add_fact_node_1", false},
		// Field names / literals — skip
		{"default", false},
		{"text", false},
		{"portrait", false},
		{"true", false},
		{"END", false},
		{"DEMO", false},
		// Too short / no letters — skip
		{"a", false},
		{"12", false},
		{"--", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isGodotText(tt.in); got != tt.want {
				t.Errorf("isGodotText(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractGodotTextStrings(t *testing.T) {
	tres := `
[gd_resource type="Resource" script_class="SproutyDialogsDialogueData" format=3 uid="uid://tc05ntgea1vj"]

[resource]
script = ExtResource("1")
graph_data = {
"DEMO": {
"dialogue_node_54": {
"character": "dialog_char_sister",
"dialog_key": "DEMO_DIALOG_54",
"node_type": "dialogue_node",
},
"text_lines": {
"default": "You should visit Dr. Lee. She is a good soul and helps anyone if you ask nicely.",
"default": "Go find her. Eighth floor. And come back and tell me how it goes, alright?",
},
}
}
`
	got := extractGodotTextStrings([]byte(tres))
	want := map[string]bool{
		"You should visit Dr. Lee. She is a good soul and helps anyone if you ask nicely.": true,
		"Go find her. Eighth floor. And come back and tell me how it goes, alright?":       true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d strings %q, want %d", len(got), got, len(want))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected string %q", s)
		}
	}
}

func TestExtractGodotTextEntriesSkipsDictionaryKeysAndKeepsLine(t *testing.T) {
	data := []byte(`
"description": "Read the note at the desk.",
"Action": "New Game",
`)
	got := extractGodotTextEntries(data)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want two values only: %+v", len(got), got)
	}
	if got[0].Text != "Read the note at the desk." || got[0].Line != 2 {
		t.Errorf("first entry = %+v, want note text on line 2", got[0])
	}
	if got[1].Text != "New Game" || got[1].Line != 3 {
		t.Errorf("second entry = %+v, want UI text on line 3", got[1])
	}
}

func TestIsGodotEditorAddon(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{path: `addons\sprouty_dialogs\editor\settings.tscn`, want: true},
		{path: `addons/sprouty_dialogs/runtime/dialog.gd`, want: false},
		{path: `content/editor/dialog.tres`, want: false},
		{path: `addons/other/plugin.gd`, want: false},
	} {
		if got := isGodotEditorAddon(tc.path); got != tc.want {
			t.Errorf("isGodotEditorAddon(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestScanGodotStringTokensHandlesCommentsQuotesAndMultiline(t *testing.T) {
	source := "# \"commented out\"\nvar single = 'A journal note'\n" +
		"var triple = \"\"\"A longer\nmultiline note\"\"\"\n" +
		`var raw = r"\n stays literal"` + "\n"
	tokens := scanGodotStringTokens(source)
	if len(tokens) != 3 {
		t.Fatalf("scanned %d string tokens, want 3: %+v", len(tokens), tokens)
	}
	if tokens[0].Text != "A journal note" || tokens[0].Line != 2 {
		t.Errorf("single-quoted token = %+v", tokens[0])
	}
	if tokens[1].Text != "A longer\nmultiline note" || tokens[1].Line != 3 {
		t.Errorf("triple-quoted token = %+v", tokens[1])
	}
	if tokens[2].Text != `\n stays literal` || tokens[2].Line != 5 {
		t.Errorf("raw token = %+v", tokens[2])
	}
}

func TestGodotUnescapeUnicodeAndLineContinuation(t *testing.T) {
	got := godotUnescape(`\u0645\u0631\u062d\u0628\u0627 \U01F642 \uD83D\uDE42 line\
continued`)
	want := "مرحبا 🙂 🙂 linecontinued"
	if got != want {
		t.Fatalf("godotUnescape() = %q, want %q", got, want)
	}
}

func TestGodotUnescape(t *testing.T) {
	in := `He said \"hello\" to me.\nNew line.`
	got := godotUnescape(in)
	if !strings.Contains(got, `"hello"`) {
		t.Errorf("unescape did not decode quotes: %q", got)
	}
	if !strings.Contains(got, "\nNew line.") {
		t.Errorf("unescape did not decode newline: %q", got)
	}
}
