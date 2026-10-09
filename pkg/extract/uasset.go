package extract

import (
	"regexp"
	"strings"
)

// ueTextLikelyDirRe matches pak directories likely to contain UI/text
// assets (widgets, data tables, dialogue, localization, HUD, missions…).
var ueTextLikelyDirRe = regexp.MustCompile(`(?i)(^|/)(ui|widgets?|interface|data|data_tables|datatables|dialogue|dialogs?|text|localization|locale|hud|menus?|mission|objective|quest|items|tutorial|help|strings?|settings|subtitles?|credits|docs?|hint)(/|$)`)

// ueAssetDirRe matches pak directories that hold graphics/audio/environment
// assets — these are skipped even when a sub-path matches the text regex.
var ueAssetDirRe = regexp.MustCompile(`(?i)(^|/)(textures?|materials?|meshes?|sounds?|audio|movies?|animations?|megascans|environments?|landscape|foliage|lights?|particles?|physics|collision|vfx|nanite|characters?|weapons?|furniture|houses?|hotels?|hospitals?|offices?|schools?|clubs?|stations?|level\d*|maps|props)(/|$)`)

// isUETextPath reports whether a pak entry path is a .uexp likely to contain
// translatable text: it must be under a text-likely directory, must not sit
// under an asset-only directory, and must not be an engine-internal file.
func isUETextPath(p string) bool {
	if strings.HasPrefix(p, "Engine/") {
		return false
	}
	if ueAssetDirRe.MatchString(p) {
		return false
	}
	return ueTextLikelyDirRe.MatchString(p)
}

// extractUexpStrings scans UE4 .uexp export data for readable text strings.
//
// UE4 stores FText properties as a "Base" history: a namespace FString, a
// key FString (a 32-char MD5 hex digest of the source), and the source
// FString (the actual text). In practice the source strings are plain UTF-8
// and the keys are 32-char hex runs, so we scan for printable ASCII runs and
// drop the hex keys. This is a heuristic (not a full .uasset property
// parser) but reliably recovers English source text from DataTable/Widget
// exports while rejecting binary noise from material/mesh exports.
func extractUexpStrings(data []byte) []string {
	var out []string
	var sb strings.Builder

	flush := func() {
		s := sb.String()
		sb.Reset()
		if isText(s) {
			out = append(out, s)
		}
	}

	for _, b := range data {
		if b >= 32 && b < 127 {
			sb.WriteByte(b)
		} else {
			flush()
		}
	}
	flush()

	return out
}

// isText reports whether s looks like translatable natural-language text
// rather than an asset name, path, hex key, or binary noise.
func isText(s string) bool {
	if len(s) < 4 {
		return false
	}
	if isHexKey(s) {
		return false
	}

	hasLetter := false
	hasSpace := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
		if c == ' ' {
			hasSpace = true
		}
		// Asset names and paths use underscores/slashes — not translatable.
		if c == '_' || c == '/' || c == '\\' {
			return false
		}
	}
	if !hasLetter {
		return false
	}
	// Natural-language text almost always contains spaces. Single-word
	// labels are rare in this engine's exports and are dominated by noise,
	// so we require a space to keep precision high.
	return hasSpace
}

// isHexKey reports whether s is a 32-character hexadecimal string (the MD5
// digest used as an FText key). These are not translatable text.
func isHexKey(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
