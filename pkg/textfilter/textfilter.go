// Package textfilter provides placeholder/tag-aware text intelligence shared
// by every extractor and the translation QA pipeline.
//
// Game strings are full of markup that must never be translated or counted
// as text:
//
//	{PW_Axe}              named placeholder
//	{GetThreshouldBVar_1} named placeholder with digits/underscore
//	[{PWSkill1}]          bracket-wrapped placeholder
//	<cf>, <Highlight>     rich-text tags
//	%s, %d, %1$s          printf-style placeholders
//	__1__                 Factorio-style placeholders (text-adjacent)
//
// The package answers three questions:
//
//  1. IsTranslatable — does this string contain real, translatable text
//     (after stripping all markup)? Pure identifiers, filenames, keys and
//     markup-only strings are rejected.
//  2. MarkupSequence — the ordered list of markup tokens, used to verify a
//     translation preserved every placeholder/tag in the same order.
//  3. QA — automatic quality checks comparing a source string with its
//     translation, producing human-readable notes (MISMATCH_PLACEHOLDERS,
//     UNBALANCED_BRACES, MISSING_TAG, LINE_COUNT_MISMATCH, …).
package textfilter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind classifies a token inside a game string.
type Kind int

const (
	// KindText is translatable text (outside or inside meaningful brackets).
	KindText Kind = iota
	// KindPlaceholder is a named or printf-style placeholder: {Name}, {0}, %s.
	KindPlaceholder
	// KindTag is a rich-text tag: <cf>, </b>, <Highlight>, <color=#fff>.
	KindTag
	// KindBracket is markup wrapped in square brackets: [{PWSkill1}], [b],
	// [url=…], [/color]. Bracket content that is real text ([Optional])
	// is emitted as KindText instead.
	KindBracket
)

// Token is one classified segment of a game string.
type Token struct {
	Kind Kind
	Text string
}

var (
	// tagRe matches XML/HTML/rich-text tags: <cf>, </b>, <Highlight>,
	// <color=#ff0000>, <br/>. Anchored matches are verified manually.
	tagRe = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9_:.!-]*(?:\s[^<>]*)?/?>`)

	// printfRe matches printf-style placeholders: %s, %d, %1$s, %02d, %.2f.
	// The conversion character is restricted to real conversions so that
	// "100% damage" or "% when casting" are not misread as placeholders.
	printfRe = regexp.MustCompile(`%(?:\d+\$)?[0-9.+#\-]*[dsfpxXeEgGciou]`)

	// bbcodeNames are known square-bracket markup tag names (BBCode and
	// common engine variants). Content like [b] or [/color] is markup;
	// [Optional] is text.
	bbcodeNames = map[string]bool{
		"b": true, "i": true, "u": true, "s": true, "sub": true, "sup": true,
		"color": true, "size": true, "font": true, "url": true, "img": true,
		"code": true, "quote": true, "spoiler": true, "center": true,
		"right": true, "left": true, "indent": true, "br": true, "hr": true,
		"highlight": true, "cf": true, "table": true, "tr": true, "td": true,
		"list": true, "li": true, "strike": true, "mark": true,
	}

	// assemblyRefRe matches Unity script references: "Class, Assembly-CSharp".
	assemblyRefRe = regexp.MustCompile(`^[A-Za-z0-9_.]+, Assembly-[A-Za-z0-9]+$`)

	// hexRunRe matches long hexadecimal runs (hashes, digests, GUID cores).
	hexRunRe = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)

	// guidRe matches GUIDs with dashes.
	guidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Tokenize splits s into text and markup tokens. Markup is never merged into
// text tokens, so downstream code can preserve it verbatim.
func Tokenize(s string) []Token {
	var toks []Token
	var sb strings.Builder

	flush := func() {
		if sb.Len() > 0 {
			toks = append(toks, Token{Kind: KindText, Text: sb.String()})
			sb.Reset()
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch r {
		case '{':
			if end := indexRune(runes, i+1, '}'); end > i+1 {
				flush()
				toks = append(toks, Token{Kind: KindPlaceholder, Text: string(runes[i : end+1])})
				i = end + 1
				continue
			}
		case '<':
			if m := tagRe.FindStringSubmatchIndex(string(runes[i:])); m != nil && m[0] == 0 {
				flush()
				toks = append(toks, Token{Kind: KindTag, Text: string(runes[i : i+m[1]])})
				i += m[1]
				continue
			}
		case '%':
			if m := printfRe.FindStringSubmatchIndex(string(runes[i:])); m != nil && m[0] == 0 {
				flush()
				toks = append(toks, Token{Kind: KindPlaceholder, Text: string(runes[i : i+m[1]])})
				i += m[1]
				continue
			}
		case '[':
			if end := indexRune(runes, i+1, ']'); end > i {
				content := string(runes[i+1 : end])
				if kind, full := classifyBracket(content); kind != KindText {
					flush()
					toks = append(toks, Token{Kind: kind, Text: full})
					i = end + 1
					continue
				}
				// Real text inside brackets: keep the brackets with the text
				// so the translator sees (and preserves) them.
				flush()
				toks = append(toks, Token{Kind: KindText, Text: string(runes[i : end+1])})
				i = end + 1
				continue
			}
		}
		sb.WriteRune(r)
		i++
	}
	flush()
	return toks
}

// indexRune returns the index of the first occurrence of target at or after
// start, or -1 if not found.
func indexRune(runes []rune, start int, target rune) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == target {
			return i
		}
	}
	return -1
}

// classifyBracket decides whether bracket content is markup or translatable
// text. It returns the token kind and the full bracket text.
func classifyBracket(content string) (Kind, string) {
	full := "[" + content + "]"
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return KindBracket, full
	}
	// Placeholder wrapped in brackets: [{PWSkill1}]
	if strings.ContainsAny(trimmed, "{}") {
		return KindBracket, full
	}
	// Closing tag: [/color]
	if strings.HasPrefix(trimmed, "/") {
		return KindBracket, full
	}
	// Attribute tag: [color=#ff0000], [url=www.x.com]
	if strings.Contains(trimmed, "=") {
		return KindBracket, full
	}
	// Known BBCode tag name: [b], [Highlight], [i:xyz]
	name := strings.ToLower(trimmed)
	if i := strings.IndexAny(name, ":. \t"); i >= 0 {
		name = name[:i]
	}
	if bbcodeNames[name] {
		return KindBracket, full
	}
	// Identifier-like content (PWSkill1, ITEM_01, 12345) is markup — it is
	// never translatable, so treating it as markup keeps QA strict.
	if isIdentifierWord(trimmed) {
		return KindBracket, full
	}
	// Everything else ([Optional], [New Item], [Press E]) is real text.
	return KindText, full
}

// IsTranslatable reports whether s contains real translatable text after all
// markup (placeholders, tags, markup brackets) is stripped. Strings that are
// pure markup, pure identifiers, filenames, paths, keys or hashes are not
// translatable.
func IsTranslatable(s string) bool {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) < 2 {
		return false
	}
	var parts []string
	for _, t := range Tokenize(s) {
		if t.Kind == KindText {
			parts = append(parts, t.Text)
		}
	}
	if len(parts) == 0 {
		return false // pure markup — nothing to translate
	}
	return isNaturalText(strings.Join(parts, " "))
}

// TranslatableText returns only the translatable text segments of s, with all
// markup stripped. Useful for previews and word counts.
func TranslatableText(s string) string {
	var parts []string
	for _, t := range Tokenize(s) {
		if t.Kind == KindText {
			parts = append(parts, t.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// MarkupSequence returns the ordered markup tokens (placeholders, tags,
// markup brackets) of s. A faithful translation must reproduce this exact
// sequence.
func MarkupSequence(s string) []string {
	var seq []string
	for _, t := range Tokenize(s) {
		if t.Kind != KindText {
			seq = append(seq, t.Text)
		}
	}
	return seq
}

// QA compares a source string with its translation and returns QA notes.
// An empty result means the translation passed all checks. Notes use stable
// machine-readable codes followed by a human-readable explanation:
//
//	MISMATCH_PLACEHOLDERS: missing {X}, extra {Y}
//	PLACEHOLDER_ORDER_DIFFERS
//	MISSING_TAG: <cf>        / EXTRA_TAG: <cf>
//	MISMATCH_BRACKETS: missing [{PWSkill1}]
//	UNBALANCED_BRACES: open=2 close=1
//	LINE_COUNT_MISMATCH: source=3 translation=2
//	EMPTY_TRANSLATION
func QA(source, translation string) []string {
	var notes []string
	tr := strings.TrimSpace(translation)
	if tr == "" {
		return []string{"EMPTY_TRANSLATION"}
	}

	srcToks := Tokenize(source)
	trToks := Tokenize(tr)

	// Compare each markup family separately for precise notes.
	for _, family := range []struct {
		kind      Kind
		label     string
		code      string
		orderCode string
	}{
		{KindPlaceholder, "placeholder", "MISMATCH_PLACEHOLDERS", "PLACEHOLDER_ORDER_DIFFERS"},
		{KindTag, "tag", "MISMATCH_TAGS", "TAG_ORDER_DIFFERS"},
		{KindBracket, "bracket", "MISMATCH_BRACKETS", "BRACKET_ORDER_DIFFERS"},
	} {
		srcSeq := tokensOf(srcToks, family.kind)
		trSeq := tokensOf(trToks, family.kind)
		if equalSeq(srcSeq, trSeq) {
			continue
		}
		srcCount, trCount := countAll(srcSeq), countAll(trSeq)
		var missing, extra []string
		for _, k := range srcSeq {
			if srcCount[k] > 0 && trCount[k] < srcCount[k] {
				missing = append(missing, k)
				srcCount[k] = 0 // report each missing token once
			}
		}
		for _, k := range trSeq {
			if trCount[k] > 0 && srcCount[k] < trCount[k] {
				extra = append(extra, k)
				trCount[k] = 0
			}
		}
		// Deduplicate while preserving order.
		missing = dedupe(missing)
		extra = dedupe(extra)
		if len(missing) == 0 && len(extra) == 0 {
			// Same multiset but different order — the markup tokens appear
			// in a different position than the source.
			notes = append(notes, family.orderCode)
			continue
		}
		msg := family.code + ":"
		if len(missing) > 0 {
			msg += " missing " + strings.Join(missing, ", ")
		}
		if len(extra) > 0 {
			if len(missing) > 0 {
				msg += ";"
			}
			msg += " extra " + strings.Join(extra, ", ")
		}
		notes = append(notes, msg)
	}

	// Brace balance in the translation (catches stray { } even when the
	// placeholder multiset happens to match).
	open, close := strings.Count(tr, "{"), strings.Count(tr, "}")
	if open != close {
		notes = append(notes, fmt.Sprintf("UNBALANCED_BRACES: open=%d close=%d", open, close))
	}

	// Line count and order preservation: a translation must have the same
	// number of lines as its source (multi-line dialogue blocks, menus).
	srcLines := strings.Count(strings.TrimRight(source, "\n"), "\n") + 1
	trLines := strings.Count(strings.TrimRight(tr, "\n"), "\n") + 1
	if srcLines != trLines {
		notes = append(notes, fmt.Sprintf("LINE_COUNT_MISMATCH: source=%d translation=%d", srcLines, trLines))
	}

	return notes
}

// QANotes joins QA notes into the entry Notes field format ("; " separated).
func QANotes(notes []string) string {
	return strings.Join(notes, "; ")
}

func tokensOf(toks []Token, kind Kind) []string {
	var out []string
	for _, t := range toks {
		if t.Kind == kind {
			out = append(out, t.Text)
		}
	}
	return out
}

func equalSeq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func countAll(seq []string) map[string]int {
	m := make(map[string]int, len(seq))
	for _, k := range seq {
		m[k]++
	}
	return m
}

func dedupe(seq []string) []string {
	seen := make(map[string]bool, len(seq))
	out := seq[:0]
	for _, s := range seq {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ── Natural-text heuristics ─────────────────────────────────────────

// engineNoiseWords are ultra-common engine/field single words ("new", "this",
// "function") that appear both in real editor sentences ("Create New
// Function") and in field dumps. They count as neutral, not unnatural: a
// string made only of noise words still fails the natural-word gate, but a
// real sentence is no longer rejected just for containing them.
var engineNoiseWords = map[string]bool{
	"true": true, "false": true, "none": true, "null": true,
	"default": true, "new": true, "self": true, "this": true,
	"void": true, "int": true, "float": true, "bool": true,
	"string": true, "struct": true, "enum": true, "class": true,
	"function": true, "vector": true, "array": true, "object": true,
	"undefined": true, "namespace": true, "public": true, "private": true,
	"static": true, "return": true, "import": true, "export": true,
	"const": true, "var": true, "let": true, "func": true,
}

// isNaturalText applies the final quality gate to markup-stripped text.
func isNaturalText(t string) bool {
	t = strings.TrimSpace(t)
	if t == "" {
		return false
	}
	if isPathOrFilename(t) || assemblyRefRe.MatchString(t) ||
		hexRunRe.MatchString(t) || guidRe.MatchString(t) {
		return false
	}
	words := strings.Fields(t)
	if len(words) == 0 {
		return false
	}
	natural, unnatural := 0, 0
	for _, w := range words {
		switch classifyWord(w) {
		case wordNatural:
			natural++
		case wordUnnatural:
			unnatural++
			// wordNeutral (camelCase brands, bare symbols, single letters)
			// counts neither way: "Vector2" and "GodotSteam v | |" carry no
			// natural word, so markup-plus-symbol noise is rejected while
			// "I am here" still passes on the strength of "am"/"here".
		}
	}
	// Real text must contain at least one natural word, and clear
	// non-text tokens (identifiers, codes, filenames) must not outnumber
	// the natural ones. This keeps "Reduce damage intake by % when casting"
	// and rejects "Item_1 Item_2 Item_3"; noise words count neither way, so
	// "Create New Function" passes on the strength of "Create".
	return natural > 0 && natural >= unnatural
}

// wordClass classifies a single whitespace-separated word.
type wordClass int

const (
	wordUnnatural wordClass = iota // identifier, code, dotted filename token
	wordNatural                    // ordinary human-language word
	wordNeutral                    // camelCase brand, bare symbol, engine noise word
)

// classifyWord reports whether a single word looks like human language
// (natural), like a technical identifier (unnatural), or is ambiguous
// (neutral): camelCase tokens such as "GodotSteam" or "YouTube" are brand
// names in real UI text but identifiers in code exports, bare symbols
// such as "|" or "100%" carry no signal either way, and engine noise words
// ("new", "this") appear in real editor sentences just as often as in dumps.
func classifyWord(w string) wordClass {
	w = strings.Trim(w, ".,!?;:\"'`()[]{}<>%*+-—–…~|@")
	if w == "" {
		return wordNeutral
	}
	// Channel/tag names: a leading '#' before a letter ("#Contracts",
	// "#Shop") is UI text; '#' anywhere else marks binary noise ("ha#}").
	if i := strings.IndexRune(w, '#'); i >= 0 {
		if i == 0 && len(w) > 1 && unicode.IsLetter(rune(w[1])) {
			w = w[1:]
		} else {
			return wordUnnatural
		}
	}
	hasLetter := false
	for _, r := range w {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return wordNeutral // "100", "%", "|", "—"
	}
	if utf8.RuneCountInString(w) == 1 {
		return wordNeutral // single letters ("V", "x") carry no signal
	}
	// Identifiers and paths: PW_Axe, npc.sister.name, Data/Tables.
	if strings.ContainsAny(w, "_/") || strings.Contains(w, "\\") {
		return wordUnnatural
	}
	// Dotted tokens: npc.sister.name, v1.2, file.txt.
	if strings.Contains(w, ".") {
		return wordUnnatural
	}
	// Mixed alphanumeric codes: BVar1, Level1, Item2.
	for _, r := range w {
		if r >= '0' && r <= '9' {
			return wordUnnatural
		}
	}
	if engineNoiseWords[strings.ToLower(w)] {
		return wordNeutral
	}
	// Binary noise: symbols that never occur inside real words once edge
	// punctuation is trimmed — "$Hqu", "MN-=", "Z~`", "B%B", "fff?l",
	// "root@OS". '&' is allowed between two letters ("Save&Exit", "R&D")
	// but is noise anywhere else ("ff&").
	if strings.ContainsAny(w, "$=^`@%?!") {
		return wordUnnatural
	}
	if strings.ContainsRune(w, '&') {
		runes := []rune(w)
		for i, r := range runes {
			if r != '&' {
				continue
			}
			if i == 0 || i == len(runes)-1 ||
				!unicode.IsLetter(runes[i-1]) || !unicode.IsLetter(runes[i+1]) {
				return wordUnnatural
			}
		}
	}
	// camelCase: GetThreshold, GodotSteam — ambiguous, treat as neutral.
	runes := []rune(w)
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			return wordNeutral
		}
	}
	return wordNatural
}

// isIdentifierWord reports whether a single token looks like a technical
// identifier (used for bracket-content classification).
func isIdentifierWord(w string) bool {
	if w == "" {
		return false
	}
	if strings.ContainsAny(w, "_/.") {
		return true
	}
	hasDigit, hasLetter := false, false
	for _, r := range w {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case unicode.IsLetter(r):
			hasLetter = true
		}
	}
	if hasDigit && hasLetter {
		return true
	}
	if !hasDigit {
		runes := []rune(w)
		for i := 1; i < len(runes); i++ {
			if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
				return true // camelCase
			}
		}
	}
	return false
}

// isPathOrFilename rejects paths by their prefix form (URLs, UNC, drive
// letters, rooted/relative paths). Bare filenames are handled per-word by
// classifyWord's dotted-token rule, so real sentences that merely mention a
// file ("ConsoleHelp.html was saved as") are not rejected wholesale.
func isPathOrFilename(t string) bool {
	if strings.Contains(t, "://") || strings.Contains(t, "res://") ||
		strings.Contains(t, "uid://") {
		return true
	}
	if strings.HasPrefix(t, `\\`) || regexp.MustCompile(`^[A-Za-z]:\\`).MatchString(t) {
		return true
	}
	return strings.HasPrefix(t, "./") || strings.HasPrefix(t, "../") ||
		strings.HasPrefix(t, "/")
}

// SortTokens is a small helper used by tests to compare token sets.
func SortTokens(toks []Token) []Token {
	out := append([]Token(nil), toks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Text < out[j].Text })
	return out
}
