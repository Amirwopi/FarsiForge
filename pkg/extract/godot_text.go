package extract

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"farsiforge/pkg/textfilter"
)

// ── Godot text-file string extraction ──────────────────────────────
//
// Godot 4 games export their content as binary resources (.res/.scn/.gdc).
// gdre_tools converts these back to text (.tres/.tscn/.gd) which contain the
// game's translatable strings as string literals:
//
//	tres:  "default": "You should visit Dr. Lee. She is a good soul..."
//	tscn:  text = "Install"
//	gd:    print("GodotSteam GDExtension updater functionality enabled")
//
// We extract quoted literals outside comments and filter non-text values (paths,
// uids, node keys, identifiers, bbcode).

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
	"type":      true,
	"value":     true,
	"data":      true,
	"index":     true,
}

type godotTextEntry struct {
	Text string
	Line int
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
		case '\'':
			sb.WriteByte('\'')
		case '\\':
			sb.WriteByte('\\')
		case 'a':
			sb.WriteByte('\a')
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case 'v':
			sb.WriteByte('\v')
		case 'u':
			r, consumed, ok := godotUnicodeEscape(s[i+1:], 4)
			if !ok {
				sb.WriteString(`\u`)
				continue
			}
			i += consumed
			if r >= 0xD800 && r <= 0xDBFF && i+2 < len(s) && s[i+1] == '\\' && s[i+2] == 'u' {
				if low, _, valid := godotUnicodeEscape(s[i+3:], 4); valid && low >= 0xDC00 && low <= 0xDFFF {
					r = utf16.DecodeRune(r, low)
					i += 6
				}
			}
			sb.WriteRune(r)
		case 'U':
			r, consumed, ok := godotUnicodeEscape(s[i+1:], 6)
			if !ok {
				sb.WriteString(`\U`)
				continue
			}
			i += consumed
			sb.WriteRune(r)
		case '\n':
			// Godot allows a backslash-newline continuation without adding a newline.
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		default:
			sb.WriteByte('\\')
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

func godotUnicodeEscape(s string, digits int) (rune, int, bool) {
	if len(s) < digits {
		return 0, 0, false
	}
	value, err := strconv.ParseUint(s[:digits], 16, 32)
	if err != nil || value > 0x10FFFF {
		return 0, 0, false
	}
	return rune(value), digits, true
}

type godotStringToken struct {
	Text string
	Line int
	End  int
}

func scanGodotStringTokens(source string) []godotStringToken {
	var tokens []godotStringToken
	line := 1
	for i := 0; i < len(source); {
		if source[i] == '#' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		raw := false
		if source[i] == 'r' && i+1 < len(source) && isGodotQuote(source[i+1]) &&
			(i == 0 || !isGodotIdentifierByte(source[i-1])) {
			raw = true
			i++
		}
		if !isGodotQuote(source[i]) {
			if source[i] == '\n' {
				line++
			}
			i++
			continue
		}

		quote := source[i]
		startLine := line
		triple := i+2 < len(source) && source[i+1] == quote && source[i+2] == quote
		delimiter := 1
		if triple {
			delimiter = 3
		}
		start := i + delimiter
		j, scanLine, closed := start, line, false
		for j < len(source) {
			if !raw && source[j] == '\\' && j+1 < len(source) {
				if source[j+1] == '\n' {
					scanLine++
					j += 2
					continue
				}
				if source[j+1] == '\r' && j+2 < len(source) && source[j+2] == '\n' {
					scanLine++
					j += 3
					continue
				}
				if source[j+1] == '\n' {
					scanLine++
				}
				j += 2
				continue
			}
			if raw && source[j] == '\\' && j+1 < len(source) && (source[j+1] == '\\' || source[j+1] == quote) {
				j += 2
				continue
			}
			if source[j] == quote && (!triple || (j+2 < len(source) && source[j+1] == quote && source[j+2] == quote)) {
				end := j
				after := j + delimiter
				value := source[start:end]
				if raw {
					value = godotUnescapeRaw(value, quote)
				} else {
					value = godotUnescape(value)
				}
				tokens = append(tokens, godotStringToken{Text: value, Line: startLine, End: after})
				line = scanLine
				i = after
				closed = true
				break
			}
			if source[j] == '\n' {
				scanLine++
			}
			j++
		}
		if !closed {
			break
		}
	}
	return tokens
}

func isGodotQuote(b byte) bool { return b == '\'' || b == '"' }

func isGodotIdentifierByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func godotUnescapeRaw(s string, quote byte) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == quote) {
			i++
			sb.WriteByte(s[i])
			continue
		}
		sb.WriteByte(s[i])
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
	entries := extractGodotTextEntries(data)
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Text
	}
	return out
}

func extractGodotTextEntries(data []byte) []godotTextEntry {
	text := string(data)
	var out []godotTextEntry
	seen := make(map[string]bool)
	for _, token := range scanGodotStringTokens(text) {
		if godotStringIsDictionaryKey(text, token.End) {
			continue
		}
		if !isGodotText(token.Text) || seen[token.Text] {
			continue
		}
		seen[token.Text] = true
		out = append(out, godotTextEntry{Text: token.Text, Line: token.Line})
	}
	return out
}

func godotStringIsDictionaryKey(text string, end int) bool {
	for end < len(text) {
		switch text[end] {
		case ' ', '\t', '\r', '\n':
			end++
		default:
			return text[end] == ':'
		}
	}
	return false
}

func isGodotEditorAddon(relPath string) bool {
	parts := strings.Split(strings.ToLower(filepath.ToSlash(filepath.Clean(relPath))), "/")
	return len(parts) >= 3 && parts[0] == "addons" && parts[2] == "editor"
}
