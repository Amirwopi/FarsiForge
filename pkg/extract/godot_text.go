package extract

import (
	"regexp"
	"strings"

	"farsiforge/pkg/textfilter"
)

// ── Godot text-file string extraction ──────────────────────────────
//
// Godot 4 games export their content as binary resources (.res/.scn/.gdc).
// gdre_tools converts these back to text (.tres/.tscn/.gd) which contain the
// game's translatable strings as double-quoted literals:
//
//	tres:  "default": "You should visit Dr. Lee. She is a good soul..."
//	tscn:  text = "Install"
//	gd:    print("GodotSteam GDExtension updater functionality enabled")
//
// We extract double-quoted strings and filter out non-text values (paths,
// uids, node keys, identifiers, bbcode).

var godotStringRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// godotSingleWordRe matches a single word that can plausibly be UI text:
// Capitalized or lowercase, letters only (plus internal apostrophe/hyphen).
// This rejects identifiers with underscores, dots, digits, or internal caps
// (dialogue_node_54, npc.sister.name, DEMO_DIALOG_54, GodotSteam, Vector2).
var godotSingleWordRe = regexp.MustCompile(`^[A-Za-z][a-z]*(?:['-][a-z]+)*$`)

// godotSkipWords are common Godot field names / literal values that are not
// translatable text.
var godotSkipWords = map[string]bool{
	"default":   true,
	"text":      true,
	"script":    true,
	"character": true,
	"portrait":  true,
	"speaker":   true,
	"resource":  true,
	"true":      true,
	"false":     true,
	"none":      true,
	"null":      true,
	"name":      true,
	"root":      true,
	"this":      true,
	"self":      true,
	"new":       true,
	"start":     true,
	"end":       true,
	"demo":      true,
}

// godotUnescape decodes the backslash escapes found in Godot string literals.
func godotUnescape(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			sb.WriteByte('\n')
		case 't':
			sb.WriteByte('\t')
		case 'r':
			sb.WriteByte('\r')
		case '"':
			sb.WriteByte('"')
		case '\\':
			sb.WriteByte('\\')
		default:
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// isGodotText reports whether s looks like translatable game text rather
// than a path, uid, key, identifier, or markup-only fragment.
//
// Any string containing '/' or '\' is rejected outright — .tres/.tscn/.gd
// files are full of res:// paths, uids and URLs, and even bbcode like
// "[url=https://x.com]…[/url]" carries the URL in the markup. Markup around
// real text is kept only when the markup itself is slash-free
// ("Click [url=www.x.com]here[/url]"); the shared textfilter gate then
// rejects pure markup, identifiers, filenames, hashes and digit noise.
func isGodotText(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}
	// Paths and uids (uid://..., res://...) are not text.
	if strings.ContainsAny(s, "/\\") {
		return false
	}

	// Shared quality gate: rejects pure markup ({PW_Axe}, [{PWSkill1}],
	// <cf></cf>), identifiers, filenames, hashes, and digit noise.
	if !textfilter.IsTranslatable(s) {
		return false
	}

	if strings.Contains(s, " ") {
		// Multi-word: require enough letters relative to other characters so
		// that format strings ("GodotSteam v%s | %s | %s") pass but noise
		// fails.
		letters, other := 0, 0
		for _, c := range s {
			if c == ' ' || c == '\t' {
				continue
			}
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				letters++
			} else {
				other++
			}
		}
		return letters >= 3 && letters*2 >= other
	}

	// Single word: must look like a word (not an identifier) and not be a
	// known field/literal name.
	if len(s) < 3 {
		return false
	}
	if !godotSingleWordRe.MatchString(s) {
		return false
	}
	return !godotSkipWords[strings.ToLower(s)]
}

// extractGodotTextStrings extracts translatable strings from the contents of
// a Godot text file (.tres/.tscn/.gd). Duplicates within the file are
// removed.
func extractGodotTextStrings(data []byte) []string {
	var out []string
	seen := make(map[string]bool)
	for _, m := range godotStringRe.FindAllString(string(data), -1) {
		inner := godotUnescape(m[1 : len(m)-1])
		if !isGodotText(inner) || seen[inner] {
			continue
		}
		seen[inner] = true
		out = append(out, inner)
	}
	return out
}
