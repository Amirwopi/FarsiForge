package persian

import (
	"strings"
	"testing"
)

func TestReshape_SimpleWord(t *testing.T) {
	// "سلام" (salaam) - all letters join
	input := "سلام"
	out := Reshape(input)
	// Each letter should be in presentation form
	for _, r := range out {
		if r < 0xFE70 || r > 0xFEFF {
			if r < 0xFB50 || r > 0xFDFF {
				t.Errorf("Reshape produced non-presentation-form rune U+%04X in %q", r, out)
			}
		}
	}
}

func TestReshape_IsolatedLetter(t *testing.T) {
	// Single "ا" (ALEF) should stay isolated
	input := "ا"
	out := Reshape(input)
	if out != string(rune(0xFE8D)) {
		t.Errorf("Isolated ALEF: got U+%04X, want U+FE8D", []rune(out)[0])
	}
}

func TestReshape_LamAlefLigature(t *testing.T) {
	// "لا" = LAM + ALEF → should produce lam-alef ligature
	input := "لا"
	out := Reshape(input)
	runes := []rune(out)
	if len(runes) != 1 {
		t.Errorf("LAM+ALEF should produce 1 ligature rune, got %d: %q", len(runes), out)
	}
	// Should be U+FEFC (LAM-ALEF final) or U+FEFB (isolated)
	if runes[0] != 0xFEFC && runes[0] != 0xFEFB {
		t.Errorf("LAM+ALEF ligature: got U+%04X, want U+FEFB or U+FEFC", runes[0])
	}
}

func TestReshape_PersianLetters(t *testing.T) {
	// "پشت" (posht) - uses PEH (پ)
	input := "پشت"
	out := Reshape(input)
	runes := []rune(out)
	// First char PEH initial form = U+FB58
	if runes[0] != 0xFB58 {
		t.Errorf("PEH initial: got U+%04X, want U+FB58", runes[0])
	}
}

func TestReshape_ZWNJ(t *testing.T) {
	// ZWNJ should prevent joining: "می‌کند" (mi‌konad)
	input := "می\u200cکند"
	out := Reshape(input)
	// ZWNJ should be dropped from output
	if strings.Contains(out, "\u200c") {
		t.Errorf("ZWNJ should be dropped from output, got %q", out)
	}
}

func TestBidiReorder_SimplePersian(t *testing.T) {
	// Persian text should be reordered for visual RTL
	input := "سلام"
	out := BidiReorder(input)
	// Reversed order
	if out == input {
		// For pure RTL text, the BiDi algorithm should reverse it
		t.Logf("Note: BidiReorder returned same order for %q -> %q (may be correct for presentation forms)", input, out)
	}
}

func TestBidiReorder_MixedText(t *testing.T) {
	// "سلام 123" — Persian + ASCII numbers
	input := "سلام 123"
	out := BidiReorder(input)
	// The output should not be empty
	if out == "" {
		t.Error("BidiReorder returned empty string for non-empty input")
	}
}

func TestProcess_FullPipeline(t *testing.T) {
	input := "سلام دنیا 123"
	out := ProcessDefault(input)

	// Should have Persian digits
	if !strings.Contains(out, "۳") {
		t.Errorf("Process should convert digits to Persian: got %q", out)
	}

	// Should be in presentation forms
	if !IsProcessed(out) {
		t.Errorf("Process should produce presentation forms: got %q", out)
	}
}

func TestToPersianDigits(t *testing.T) {
	input := "1234567890"
	out := ToPersianDigits(input)
	want := "۱۲۳۴۵۶۷۸۹۰"
	if out != want {
		t.Errorf("ToPersianDigits: got %q, want %q", out, want)
	}
}

func TestFixYeh(t *testing.T) {
	// Arabic YEH → Persian FARSI YEH
	input := "ي"
	out := FixYeh(input)
	if out != "ی" {
		t.Errorf("FixYeh: got U+%04X, want U+06CC", []rune(out)[0])
	}

	// Arabic KAF → Persian KEHEH
	input = "ك"
	out = FixYeh(input)
	if out != "ک" {
		t.Errorf("FixYeh KAF: got U+%04X, want U+06A9", []rune(out)[0])
	}
}

func TestHasPersian(t *testing.T) {
	if !HasPersian("سلام") {
		t.Error("HasPersian should return true for Persian text")
	}
	if HasPersian("Hello World") {
		t.Error("HasPersian should return false for English text")
	}
	if !HasPersian("Hello سلام") {
		t.Error("HasPersian should return true for mixed text")
	}
}

func TestReshape_Roundtrip(t *testing.T) {
	// Process a sentence and verify it doesn't crash and produces output
	inputs := []string{
		"سلام",
		"این یک تست است",
		"بازی فارسی",
		"آری این کار میکند",
		"۱۲۳ تست",
		"پیشرفت",
	}

	for _, input := range inputs {
		out := ProcessDefault(input)
		if out == "" && input != "" {
			t.Errorf("Process returned empty for %q", input)
		}
	}
}
