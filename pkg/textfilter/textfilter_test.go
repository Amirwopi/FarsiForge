package textfilter

import (
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		in   string
		want []Token
	}{
		{
			// The % here is a literal percent sign after the placeholder —
			// not a printf conversion — so it stays part of the text.
			in: "Reduce damage intake by {GetThreshouldBVar_1}% when casting",
			want: []Token{
				{KindText, "Reduce damage intake by "},
				{KindPlaceholder, "{GetThreshouldBVar_1}"},
				{KindText, "% when casting"},
			},
		},
	}
	for _, c := range cases {
		got := Tokenize(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("Tokenize(%q): got %d tokens, want %d: %+v", c.in, len(got), len(c.want), got)
		}
		for i := range got {
			if got[i].Kind != c.want[i].Kind || got[i].Text != c.want[i].Text {
				t.Errorf("Tokenize(%q)[%d] = {%d %q}, want {%d %q}", c.in, i, got[i].Kind, got[i].Text, c.want[i].Kind, c.want[i].Text)
			}
		}
	}
}

func TestTokenizeMarkupExamples(t *testing.T) {
	// {TitleCommon}<cf>Text<cf>{TitleExtra}
	toks := Tokenize("{TitleCommon}<cf>Text<cf>{TitleExtra}")
	var kinds []Kind
	var texts []string
	for _, tk := range toks {
		kinds = append(kinds, tk.Kind)
		texts = append(texts, tk.Text)
	}
	wantKinds := []Kind{KindPlaceholder, KindTag, KindText, KindTag, KindPlaceholder}
	wantTexts := []string{"{TitleCommon}", "<cf>", "Text", "<cf>", "{TitleExtra}"}
	for i := range wantKinds {
		if i >= len(kinds) || kinds[i] != wantKinds[i] || texts[i] != wantTexts[i] {
			t.Fatalf("got %+v, want %v / %v", toks, wantKinds, wantTexts)
		}
	}

	// [{PWSkill1}] is a single bracket-markup token.
	toks = Tokenize("[{PWSkill1}] Press to activate")
	if len(toks) != 2 || toks[0].Kind != KindBracket || toks[0].Text != "[{PWSkill1}]" ||
		toks[1].Kind != KindText {
		t.Fatalf("bracket placeholder tokenize wrong: %+v", toks)
	}

	// [Optional] is real text inside brackets.
	toks = Tokenize("[Optional] Enable autosave")
	if len(toks) != 2 || toks[0].Kind != KindText || toks[0].Text != "[Optional]" {
		t.Fatalf("[Optional] should be text: %+v", toks)
	}

	// printf placeholders.
	toks = Tokenize("%s killed %2$d enemies")
	if len(toks) != 4 ||
		toks[0].Kind != KindPlaceholder || toks[0].Text != "%s" ||
		toks[2].Kind != KindPlaceholder || toks[2].Text != "%2$d" {
		t.Fatalf("printf tokenize wrong: %+v", toks)
	}
}

func TestIsTranslatable(t *testing.T) {
	yes := []string{
		"Reduce damage intake by {GetThreshouldBVar_1}% when casting",
		"{TitleCommon}<cf>Text<cf>{TitleExtra}",
		"[{PWSkill1}] Press to activate",
		"START MASSAGE",
		"Essential Oil Table",
		"A~~ You're hurting me on purpose, right?",
		"You should visit Dr. Lee. She is a good soul and helps anyone if you ask nicely.",
		"Install",
		"Cancel",
		"Click [url=www.example.com]here[/url] for more",
		"Reduce damage intake by %s%% when casting",
		"HP",
		"OK",
		// IRC channel names: leading '#' before a letter is UI text.
		"#Contracts",
		"#Shop",
		// '&' between letters is real text ("Save&Exit", "R&D").
		"Save&Exit",
		"R&D",
		"Save & Exit",
		// Format strings with a translatable word around the placeholder.
		"Key({0})",
		"Char({0})",
		// Hacker Simulator FTP/tutorial strings.
		"501 Usage: cat [folder]/[file]",
		"550 File \"{file_name}\" not found",
		// Engine.locres editor sentences (former false positives): noise
		// words and dotted filename mentions no longer veto real sentences.
		"'{0}' must exist and contain a DefaultEngine.ini.",
		"Create New Function",
		"Create a new object",
		"Static Vector Field",
		"Import new LOD",
		"Enable/disable this column",
		"ConsoleHelp.html was saved as",
		"Creates a new function in this FunctionDirector.",
		"FreeImage.dll couldn't be found. Texture resizing won't be done.",
		"Opens the curve table's source data file in an external editor. It will search using the following extensions: .xls/.xlsm/.csv/.json",
	}
	no := []string{
		"{PW_Axe}",                         // pure placeholder
		"[{PWSkill1}]",                     // pure bracket markup
		"<cf></cf>",                        // pure tags
		"DEMO_DIALOG_54",                   // key
		"npc.sister.name",                  // dotted identifier
		"dialogue_node_54",                 // snake identifier
		"GodotSteam",                       // camelCase identifier
		"Assets/Textures/rock.png",         // path
		"foo.uasset",                       // filename
		"Class, Assembly-CSharp",           // assembly ref
		"d41d8cd98f00b204e9800998ecf8427e", // md5 hex
		"12345",                            // number
		"{GetThreshouldBVar_1}",            // pure placeholder
		"Item_1 Item_2 Item_3",             // identifier words only
		"",                                 // empty
		"a",                                // too short
		// Binary noise leaked from raw .uexp scans (Hacker Simulator).
		"$Hqu",
		"?!V;?",
		"<MN-=",
		"})@H",
		"b`nm",
		"ff&?",
		"B%B@",
		"<Z~`<",
		"$|Fw",
		"ha#}",
		"#<fff?",
		"<fff?l",
		"root@OS:~$", // terminal prompt — '@' embedded, not translatable
		// Engine chrome from official locres tables.
		"1080p24",
		"N/A",
		"Date/Time",
		"{x}x{y}", // resolution format — no translatable word
		// Bare filenames are still rejected per-word (dotted-token rule).
		"readme.txt",
		"data.json",
		"screenshot.png",
		// Pure engine noise words carry no natural word.
		"New New New",
		"true false null",
		"Item_1",
	}
	for _, s := range yes {
		if !IsTranslatable(s) {
			t.Errorf("IsTranslatable(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if IsTranslatable(s) {
			t.Errorf("IsTranslatable(%q) = true, want false", s)
		}
	}
}

func TestMarkupSequence(t *testing.T) {
	seq := MarkupSequence("{TitleCommon}<cf>Text<cf>{TitleExtra}")
	want := []string{"{TitleCommon}", "<cf>", "<cf>", "{TitleExtra}"}
	if len(seq) != len(want) {
		t.Fatalf("MarkupSequence len = %d, want %d: %v", len(seq), len(want), seq)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Errorf("seq[%d] = %q, want %q", i, seq[i], want[i])
		}
	}
}

func TestQAUserExample(t *testing.T) {
	src := "Reduce damage intake by {GetThreshouldBVar_1}% when casting"
	good := "در حین اجرای مهارت، دریافت آسیب را به میزان {GetThreshouldBVar_1}% کاهش می‌دهد"
	if notes := QA(src, good); len(notes) != 0 {
		t.Errorf("QA(good translation) = %v, want empty", notes)
	}

	// Missing placeholder.
	bad := "در حین اجرای مهارت، دریافت آسیب را کاهش می‌دهد"
	notes := QA(src, bad)
	if len(notes) == 0 || !strings.Contains(notes[0], "MISMATCH_PLACEHOLDERS") ||
		!strings.Contains(notes[0], "{GetThreshouldBVar_1}") {
		t.Errorf("QA(missing placeholder) = %v, want MISMATCH_PLACEHOLDERS with token", notes)
	}
}

func TestQAChecks(t *testing.T) {
	// Tag missing.
	notes := QA("{TitleCommon}<cf>Text<cf>{TitleExtra}", "{TitleCommon}Text{TitleExtra}")
	if len(notes) == 0 || !strings.Contains(notes[0], "MISMATCH_TAGS") {
		t.Errorf("QA(tag missing) = %v, want MISMATCH_TAGS", notes)
	}

	// Bracket markup missing.
	notes = QA("[{PWSkill1}] Press", "Press")
	if len(notes) == 0 || !strings.Contains(notes[0], "MISMATCH_BRACKETS") {
		t.Errorf("QA(bracket missing) = %v, want MISMATCH_BRACKETS", notes)
	}

	// Unbalanced braces: the unclosed {X is not a placeholder token, so we
	// get both a placeholder mismatch and the brace-balance note.
	notes = QA("Deal {X} damage", "اعمال {X آسیب")
	joined := strings.Join(notes, "; ")
	if !strings.Contains(joined, "UNBALANCED_BRACES") || !strings.Contains(joined, "MISMATCH_PLACEHOLDERS") {
		t.Errorf("QA(unbalanced) = %v, want UNBALANCED_BRACES + MISMATCH_PLACEHOLDERS", notes)
	}

	// Line count mismatch.
	notes = QA("Line one\nLine two\nLine three", "خط یک\nخط دو")
	if len(notes) == 0 || !strings.Contains(notes[0], "LINE_COUNT_MISMATCH") {
		t.Errorf("QA(line count) = %v, want LINE_COUNT_MISMATCH", notes)
	}

	// Same line count passes.
	notes = QA("Line one\nLine two", "خط یک\nخط دو")
	if len(notes) != 0 {
		t.Errorf("QA(same lines) = %v, want empty", notes)
	}

	// Empty translation.
	notes = QA("Hello", "   ")
	if len(notes) != 1 || notes[0] != "EMPTY_TRANSLATION" {
		t.Errorf("QA(empty) = %v, want EMPTY_TRANSLATION", notes)
	}

	// Order differs (same multiset).
	notes = QA("{A} text {B} more", "متن {B} بیشتر {A}")
	found := false
	for _, n := range notes {
		if strings.Contains(n, "PLACEHOLDER_ORDER_DIFFERS") {
			found = true
		}
	}
	if !found {
		t.Errorf("QA(order differs) = %v, want PLACEHOLDER_ORDER_DIFFERS", notes)
	}
}

func TestTranslatableText(t *testing.T) {
	got := TranslatableText("{TitleCommon}<cf>Text<cf>{TitleExtra}")
	if got != "Text" {
		t.Errorf("TranslatableText = %q, want %q", got, "Text")
	}
	got = TranslatableText("Reduce damage intake by {GetThreshouldBVar_1}% when casting")
	if !strings.Contains(got, "Reduce damage intake by") {
		t.Errorf("TranslatableText = %q", got)
	}
}
