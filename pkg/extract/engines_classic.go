package extract

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
	"farsiforge/pkg/textfilter"
)

// ── Valve KeyValues parser (GoldSrc / Source 2) ────────────────────
//
// Format:
//
//	"lang"
//	{
//		"Language" "English"
//		"Tokens"
//		{
//			"KEY"   "Value"
//		}
//	}
//
// Comments use // and /* */. Strings are double-quoted with \" escapes.

var kvTokenRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"|([{}])`)

// parseKeyValues extracts "key" "value" pairs from Valve KeyValues text.
// The final element of each "value" string is treated as the translatable
// text; keys are used as entry paths.
func parseKeyValues(data []byte) [][2]string {
	tokens := kvTokenRe.FindAllSubmatch(data, -1)
	var pairs [][2]string
	var stack []string
	expectValue := false
	var pendingKey string

	for _, m := range tokens {
		if m[2] != nil { // brace
			switch m[2][0] {
			case '{':
				stack = append(stack, pendingKey)
				pendingKey = ""
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
			expectValue = false
			continue
		}
		val := unquoteKV(string(m[1]))
		if expectValue {
			key := pendingKey
			if len(stack) > 0 && stack[len(stack)-1] != "Tokens" {
				key = strings.Join(append(append([]string{}, stack...), pendingKey), ".")
			}
			pairs = append(pairs, [2]string{key, val})
			expectValue = false
			pendingKey = ""
		} else {
			pendingKey = val
			expectValue = true
		}
	}
	return pairs
}

func unquoteKV(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// isKVText rejects non-translatable values (language names are harmless, but
// token refs, identifiers, filenames and markup-only strings are skipped).
// The check is placeholder-aware (pkg/textfilter): values like
// "Insert __1__ into __2__" or "Press {button} to continue" are kept.
func isKVText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return textfilter.IsTranslatable(s)
}

// ── Lua table parser (Project Zomboid) ─────────────────────────────
//
//	Table_EN = {
//	    Key_name = "Value with <LINE> markup",
//	}

var zomboidPairRe = regexp.MustCompile(`(\w+)\s*=\s*"((?:[^"\\]|\\.)*)"`)

func parseZomboidLua(data []byte) [][2]string {
	var pairs [][2]string
	for _, m := range zomboidPairRe.FindAllSubmatch(data, -1) {
		pairs = append(pairs, [2]string{string(m[1]), unquoteKV(string(m[2]))})
	}
	return pairs
}

// ── Factorio locale parser ─────────────────────────────────────────
//
//	[section]
//	key=Value
//	key=Value with __1__ placeholders

func parseFactorioLocale(data []byte) [][2]string {
	var pairs [][2]string
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 1 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if section != "" {
			key = section + "." + key
		}
		if val != "" {
			pairs = append(pairs, [2]string{key, val})
		}
	}
	return pairs
}

// ── SAGE .big archive parser ───────────────────────────────────────
//
// CnC Generals / Zero Hour / RA3 store files in .big archives:
//
//	magic  "BIGF" | "BIG4"           (4 bytes)
//	u32 BE total size
//	u32 BE entry count
//	entries: u32 BE offset, u32 BE size, null-terminated name
//	data follows
//
// Localization text lives in .csf files (RA3) or .csf/.str (Generals).

type bigEntry struct {
	offset uint32
	size   uint32
	name   string
}

func parseBig(data []byte) ([]bigEntry, bool) {
	if len(data) < 16 {
		return nil, false
	}
	magic := string(data[:4])
	if magic != "BIGF" && magic != "BIG4" {
		return nil, false
	}
	count := binary.BigEndian.Uint32(data[8:12])
	if count == 0 || count > 100000 {
		return nil, false
	}
	pos := 16
	entries := make([]bigEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		if pos+8 > len(data) {
			return nil, false
		}
		offset := binary.BigEndian.Uint32(data[pos : pos+4])
		size := binary.BigEndian.Uint32(data[pos+4 : pos+8])
		pos += 8
		end := pos
		for end < len(data) && data[end] != 0 {
			end++
		}
		if end >= len(data) {
			return nil, false
		}
		name := string(data[pos:end])
		pos = end + 1
		entries = append(entries, bigEntry{offset: offset, size: size, name: name})
	}
	return entries, true
}

// ── SAGE .csf string file parser ───────────────────────────────────
//
// Two variants exist:
//
// Generals / Zero Hour / RA3 (" FSC"):
//
//	" FSC"  magic               (4 bytes)
//	u32 LE  version (=3)
//	u32 LE  numLabels
//	u32 LE  numStrings
//	u32 LE  unknown
//	u32 LE  unknown
//	then numLabels entries:
//	  " LBL"  magic             (4 bytes)
//	  u32 LE  pair count (=1)
//	  u32 LE  label length
//	  label   ASCII bytes (length-prefixed)
//	  " RTS" | "WRTS"           (4 bytes)
//	  u32 LE  value length (UTF-16 code units)
//	  UTF-16LE value
//
// Classic Red Alert 2 ("CSF "):
//
//	"CSF "  magic               (4 bytes)
//	u32 LE  version
//	u32 LE  numLabels
//	u32 LE  numStrings
//	u32 LE  unknown
//	u32 LE  language id
//	then numLabels entries:
//	  "LBL "  magic             (4 bytes)
//	  u32 LE  pair count (=1)
//	  label   null-terminated ASCII
//	  "RTS " | "WRTS "          (4 bytes)
//	  u32 LE  value length (UTF-16 code units)
//	  UTF-16LE value

func parseCsf(data []byte) [][2]string {
	if len(data) < 24 {
		return nil
	}
	switch string(data[:4]) {
	case " FSC":
		return parseCsfFsc(data)
	case "CSF ":
		return parseCsfCsf(data)
	}
	// Some RA3 .csf entries carry a few stray bytes before the real magic —
	// locate the magic within the first 64 bytes and parse from there.
	for i := 1; i < len(data)-4 && i < 64; i++ {
		if string(data[i:i+4]) == " FSC" {
			return parseCsfFsc(data[i:])
		}
		if string(data[i:i+4]) == "CSF " {
			return parseCsfCsf(data[i:])
		}
	}
	return nil
}

func parseCsfFsc(data []byte) [][2]string {
	var pairs [][2]string
	numLabels := binary.LittleEndian.Uint32(data[8:12])
	if numLabels == 0 || numLabels > 500000 {
		return nil
	}
	pos := 24
	for i := uint32(0); i < numLabels; i++ {
		if pos+12 > len(data) {
			break
		}
		if string(data[pos:pos+4]) != " LBL" {
			break
		}
		pos += 8 // " LBL" + pair count
		lblLen := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if lblLen < 0 || pos+lblLen > len(data) {
			break
		}
		label := string(data[pos : pos+lblLen])
		pos += lblLen
		if pos+8 > len(data) {
			break
		}
		valMagic := string(data[pos : pos+4])
		pos += 4
		if valMagic != " RTS" && valMagic != "WRTS" && valMagic != "RTS " {
			break
		}
		valLen := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if valLen < 0 || pos+valLen*2 > len(data) {
			break
		}
		val := decodeCsfUtf16(data[pos : pos+valLen*2])
		pos += valLen * 2
		if val != "" {
			pairs = append(pairs, [2]string{label, val})
		}
	}
	return pairs
}

func parseCsfCsf(data []byte) [][2]string {
	var pairs [][2]string
	numLabels := binary.LittleEndian.Uint32(data[8:12])
	pos := 24
	for i := uint32(0); i < numLabels; i++ {
		if pos+8 > len(data) {
			break
		}
		magic := string(data[pos : pos+4])
		pos += 4
		if magic != "LBL " && magic != "LBL" {
			break
		}
		pos += 4 // pair count
		labelStart := pos
		for pos < len(data) && data[pos] != 0 {
			pos++
		}
		if pos >= len(data) {
			break
		}
		label := string(data[labelStart:pos])
		pos++ // skip NUL
		if pos+8 > len(data) {
			break
		}
		valMagic := string(data[pos : pos+4])
		pos += 4
		if valMagic != "RTS " && valMagic != "WRTS" {
			break
		}
		valLen := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
		pos += 4
		if valLen < 0 || pos+valLen*2 > len(data) {
			break
		}
		val := decodeCsfUtf16(data[pos : pos+valLen*2])
		pos += valLen * 2
		if val != "" {
			pairs = append(pairs, [2]string{label, val})
		}
	}
	return pairs
}

func decodeCsfUtf16(raw []byte) string {
	var sb strings.Builder
	for j := 0; j+1 < len(raw); j += 2 {
		code := binary.LittleEndian.Uint16(raw[j : j+2])
		if code == 0 {
			continue
		}
		sb.WriteRune(rune(code))
	}
	return sb.String()
}

// ── Classic / custom-engine extractors ─────────────────────────────

// GoldSrcExtractor extracts text from Valve *_english.txt KeyValues files
// (Counter-Strike 1.6 and other GoldSrc games).
type GoldSrcExtractor struct{}

func (e *GoldSrcExtractor) SupportedEngine() string { return "goldsrc" }
func (e *GoldSrcExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true, NeedsExternalTool: false}
}

func (e *GoldSrcExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	textFiles := scanner.WalkDir(info.GameRoot, 5, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return strings.HasSuffix(base, "_english.txt") ||
			strings.HasSuffix(base, "_language.txt")
	})
	for _, tf := range textFiles {
		e.addKeyValuesFile(tf, info.GameRoot, proj)
	}
	return nil
}

func (e *GoldSrcExtractor) addKeyValuesFile(path, root string, proj *core.Project) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	relPath, _ := filepath.Rel(root, path)
	count := 0
	for _, p := range parseKeyValues(data) {
		if !isKVText(p[1]) {
			continue
		}
		proj.AddEntry(core.StringEntry{Source: p[1], File: relPath, Path: p[0], Context: "GoldSrc"})
		count++
	}
	if count > 0 {
		proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
	}
}

// Source2Extractor extracts text from Valve Source 2 *_english.txt
// localization files (Deadlock, CS2).
type Source2Extractor struct{}

func (e *Source2Extractor) SupportedEngine() string { return "source2" }
func (e *Source2Extractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true, NeedsExternalTool: false}
}

func (e *Source2Extractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	textFiles := scanner.WalkDir(info.GameRoot, 6, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return strings.HasSuffix(base, "_english.txt") ||
			strings.HasSuffix(base, "_english.vdf") ||
			base == "closecaption_english.txt"
	})
	for _, tf := range textFiles {
		e.addKeyValuesFile(tf, info.GameRoot, proj)
	}
	return nil
}

func (e *Source2Extractor) addKeyValuesFile(path, root string, proj *core.Project) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	relPath, _ := filepath.Rel(root, path)
	count := 0
	for _, p := range parseKeyValues(data) {
		if !isKVText(p[1]) {
			continue
		}
		proj.AddEntry(core.StringEntry{Source: p[1], File: relPath, Path: p[0], Context: "Source2"})
		count++
	}
	if count > 0 {
		proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
	}
}

// FactorioExtractor extracts text from Factorio locale .cfg files.
type FactorioExtractor struct{}

func (e *FactorioExtractor) SupportedEngine() string { return "factorio" }
func (e *FactorioExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true, NeedsExternalTool: false}
}

func (e *FactorioExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	// Locale files live at <root>\data\**\locale\en\*.cfg — search for .cfg
	// files whose path contains locale/en (WalkDir visits files only).
	cfgFiles := scanner.WalkDir(info.GameRoot, 8, func(p string) bool {
		if !strings.EqualFold(filepath.Ext(p), ".cfg") {
			return false
		}
		lower := strings.ToLower(p)
		return strings.Contains(lower, `locale\en\`) || strings.Contains(lower, `locale/en/`)
	})
	for _, cf := range cfgFiles {
		data, err := os.ReadFile(cf)
		if err != nil {
			continue
		}
		relPath, _ := filepath.Rel(info.GameRoot, cf)
		count := 0
		for _, p := range parseFactorioLocale(data) {
			proj.AddEntry(core.StringEntry{Source: p[1], File: relPath, Path: p[0], Context: "Factorio"})
			count++
		}
		if count > 0 {
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
		}
	}
	return nil
}

// ZomboidExtractor extracts text from Project Zomboid Lua translation tables.
type ZomboidExtractor struct{}

func (e *ZomboidExtractor) SupportedEngine() string { return "zomboid" }
func (e *ZomboidExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true, NeedsExternalTool: false}
}

func (e *ZomboidExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	// media/lua/shared/Translate/EN/**/*.txt and media/UI/en/*.txt
	enFiles := scanner.WalkDir(info.GameRoot, 8, func(p string) bool {
		lower := strings.ToLower(p)
		return strings.HasSuffix(lower, ".txt") &&
			(strings.Contains(lower, `translate\en`) ||
				strings.Contains(lower, `translate/en`) ||
				strings.Contains(lower, `ui\en`) ||
				strings.Contains(lower, `ui/en`) ||
				strings.HasSuffix(lower, "_en.txt"))
	})
	for _, tf := range enFiles {
		data, err := os.ReadFile(tf)
		if err != nil {
			continue
		}
		relPath, _ := filepath.Rel(info.GameRoot, tf)
		count := 0
		for _, p := range parseZomboidLua(data) {
			if !isKVText(p[1]) {
				continue
			}
			proj.AddEntry(core.StringEntry{Source: p[1], File: relPath, Path: p[0], Context: "Zomboid"})
			count++
		}
		if count > 0 {
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
		}
	}
	return nil
}

// SAGEExtractor extracts text from CnC Generals / Zero Hour / RA3 .big
// archives: unpack .csf string files and emit label→text pairs.
type SAGEExtractor struct{}

func (e *SAGEExtractor) SupportedEngine() string { return "sage" }
func (e *SAGEExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true, NeedsExternalTool: false}
}

func (e *SAGEExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	// Localization .big files (skip audio/movies and non-English langs)
	bigFiles := scanner.WalkDir(info.GameRoot, 3, func(p string) bool {
		if !strings.EqualFold(filepath.Ext(p), ".big") {
			return false
		}
		base := strings.ToLower(filepath.Base(p))
		for _, skip := range []string{"audio", "movie", "french", "german", "italian", "spanish", "russian", "polish", "chinese", "korean", "japanese", "portuguese"} {
			if strings.Contains(base, skip) {
				return false
			}
		}
		return true
	})

	for _, bf := range bigFiles {
		data, err := os.ReadFile(bf)
		if err != nil {
			log.Warn("Failed to read big", "file", bf, "error", err)
			continue
		}
		entries, ok := parseBig(data)
		if !ok {
			log.Warn("Not a SAGE .big file", "file", bf)
			continue
		}
		relPath, _ := filepath.Rel(info.GameRoot, bf)
		count := 0
		for _, ent := range entries {
			lower := strings.ToLower(ent.name)
			if ent.offset+ent.size > uint32(len(data)) {
				continue
			}
			inner := data[ent.offset : ent.offset+ent.size]
			var pairs [][2]string
			switch {
			case strings.HasSuffix(lower, ".csf"):
				pairs = parseCsf(inner)
			case strings.HasSuffix(lower, ".str"):
				// .str: line-oriented "KEY" "value" format
				pairs = parseStr(inner)
			case strings.HasSuffix(lower, ".manifest"):
				// RA3 UI manifests interleave binary + ASCII label runs.
				texts := extractManifestText(inner)
				for i, t := range texts {
					proj.AddEntry(core.StringEntry{
						Source:  t,
						File:    relPath + "/" + ent.name,
						Path:    fmt.Sprintf("manifest_%d", i),
						Context: "SAGE",
					})
					count++
				}
				continue
			default:
				continue
			}
			for _, p := range pairs {
				proj.AddEntry(core.StringEntry{Source: p[1], File: relPath + "/" + ent.name, Path: p[0], Context: "SAGE"})
				count++
			}
		}
		if count > 0 {
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
		}
	}
	return nil
}

var strLineRe = regexp.MustCompile(`^\s*"([^"]*)"\s*"((?:[^"\\]|\\.)*)"`)

// parseStr parses Generals-style .str files: lines of "KEY" "value".
func parseStr(data []byte) [][2]string {
	var pairs [][2]string
	for _, line := range strings.Split(string(data), "\n") {
		m := strLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pairs = append(pairs, [2]string{m[1], unquoteKV(m[2])})
	}
	return pairs
}

// extractManifestText scans a RA3 .manifest blob for readable text runs.
// Manifests interleave ASCII UI labels with binary data, so we collect
// printable runs and keep those that look like words/phrases.
func extractManifestText(data []byte) []string {
	var out []string
	seen := make(map[string]bool)
	var sb strings.Builder
	flush := func() {
		s := strings.TrimSpace(sb.String())
		sb.Reset()
		if len(s) < 4 || seen[s] {
			return
		}
		// Identifiers/paths (underscores, slashes, backslashes) are not
		// translatable text.
		if strings.ContainsAny(s, "_/\\") {
			return
		}
		letters, alnum, other := 0, 0, 0
		for _, c := range s {
			switch {
			case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
				letters++
				alnum++
			case (c >= '0' && c <= '9'):
				alnum++
			case c == ' ' || c == '\t':
				// word separator, not counted
			default:
				other++
			}
		}
		if letters < 2 {
			return
		}
		total := alnum + other
		if total == 0 || alnum*100 < total*60 {
			return
		}
		if !strings.Contains(s, " ") && letters < 6 {
			return
		}
		// Final shared gate: reject identifiers, filenames and markup-only
		// runs that slip past the manifest-specific heuristics.
		if !textfilter.IsTranslatable(s) {
			return
		}
		seen[s] = true
		out = append(out, s)
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
