package persian

// ArabicReshaper converts Arabic/Persian text from logical order with
// base (isolated) Unicode characters into the "presentation forms" 
// (U+FB50–U+FDFF, U+FE70–U+FEFF) with correct joining, so that engines
// without an Arabic shaping engine render joined glyphs correctly.
//
// This is the standard technique used in Persian game localization:
// text is pre-shaped and BiDi-reordered before being stored in game files.

// joiningType classifies a Unicode Arabic-script character for the
// Arabic joining algorithm (UAX#8 / Unicode 16.0).
type joiningType uint8

const (
	jtU joiningType = iota // non-joining (e.g. space, punctuation)
	jtL                    // left-joining   (causes right neighbour to join on its right side)
	jtR                    // right-joining  (causes left neighbour to join on its left side)
	jtD                    // dual-joining   (joins on both sides)
	jtC                    // join-causing   (ZWNJ/ZWJ behaviour)
	jtT                    // transparent    (combining marks, ZWJ/ZWNJ are transparent-ish)
)

// arChar holds the joining type and the four presentation-form codepoints
// (isolated, final, initial, medial) for an Arabic-script letter.
// If a form is 0, the isolated form is used (no dedicated presentation form).
type arChar struct {
	jt       joiningType
	isolated rune
	final    rune
	initial  rune
	medial   rune
}

// arMap maps base Arabic-script letters to their presentation-form data.
var arMap = map[rune]arChar{
	// ── Basic Arabic letters (Forms-B, U+FE70–U+FEFF) ──────────────
	0x0621: {jtU, 0xFE80, 0, 0, 0},             // HAMZA
	0x0622: {jtR, 0xFE81, 0xFE82, 0, 0},         // ALEF WITH MADDA ABOVE
	0x0623: {jtR, 0xFE83, 0xFE84, 0, 0},         // ALEF WITH HAMZA ABOVE
	0x0624: {jtR, 0xFE85, 0xFE86, 0, 0},         // WAW WITH HAMZA ABOVE
	0x0625: {jtR, 0xFE87, 0xFE88, 0, 0},         // ALEF WITH HAMZA BELOW
	0x0626: {jtD, 0xFE89, 0xFE8A, 0xFE8B, 0xFE8C}, // YEH WITH HAMZA ABOVE
	0x0627: {jtR, 0xFE8D, 0xFE8E, 0, 0},         // ALEF
	0x0628: {jtD, 0xFE8F, 0xFE90, 0xFE91, 0xFE92}, // BEH
	0x0629: {jtR, 0xFE93, 0xFE94, 0, 0},         // TEH MARBUTA
	0x062A: {jtD, 0xFE95, 0xFE96, 0xFE97, 0xFE98}, // TEH
	0x062B: {jtD, 0xFE99, 0xFE9A, 0xFE9B, 0xFE9C}, // THEH
	0x062C: {jtD, 0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0}, // JEEM
	0x062D: {jtD, 0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4}, // HAH
	0x062E: {jtD, 0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8}, // KHAH
	0x062F: {jtR, 0xFEA9, 0xFEAA, 0, 0},         // DAL
	0x0630: {jtR, 0xFEAB, 0xFEAC, 0, 0},         // THAL
	0x0631: {jtR, 0xFEAD, 0xFEAE, 0, 0},         // REH
	0x0632: {jtR, 0xFEAF, 0xFEB0, 0, 0},         // ZAIN
	0x0633: {jtD, 0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4}, // SEEN
	0x0634: {jtD, 0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8}, // SHEEN
	0x0635: {jtD, 0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC}, // SAD
	0x0636: {jtD, 0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0}, // DAD
	0x0637: {jtD, 0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4}, // TAH
	0x0638: {jtD, 0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8}, // ZAH
	0x0639: {jtD, 0xFEC9, 0xFECA, 0xFECB, 0xFECC}, // AIN
	0x063A: {jtD, 0xFECD, 0xFECE, 0xFECF, 0xFED0}, // GHAIN
	0x0641: {jtD, 0xFED1, 0xFED2, 0xFED3, 0xFED4}, // FEH
	0x0642: {jtD, 0xFED5, 0xFED6, 0xFED7, 0xFED8}, // QAF
	0x0643: {jtD, 0xFED9, 0xFEDA, 0xFEDB, 0xFEDC}, // KAF
	0x0644: {jtD, 0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0}, // LAM
	0x0645: {jtD, 0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4}, // MEEM
	0x0646: {jtD, 0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8}, // NOON
	0x0647: {jtD, 0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC}, // HEH
	0x0648: {jtR, 0xFEED, 0xFEEE, 0, 0},         // WAW
	0x0649: {jtD, 0xFEEF, 0xFEF0, 0xFBE8, 0xFBE9}, // ALEF MAKSURA (Persian YEH in some fonts)
	0x064A: {jtD, 0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4}, // YEH

	// ── Persian-specific letters ───────────────────────────────────
	0x067E: {jtD, 0xFB56, 0xFB57, 0xFB58, 0xFB59}, // PEH (پ)
	0x0686: {jtD, 0xFB7A, 0xFB7B, 0xFB7C, 0xFB7D}, // TCHEH (چ)
	0x0698: {jtR, 0xFB8A, 0xFB8B, 0, 0},           // JEH (ژ)
	0x06AF: {jtD, 0xFB94, 0xFB95, 0xFB96, 0xFB97}, // GAF (گ)
	0x06A9: {jtD, 0xFB8E, 0xFB8F, 0xFB90, 0xFB91}, // KEHEH (ک) — Persian Kaf
	0x06CC: {jtD, 0xFBFC, 0xFBFD, 0xFBFE, 0xFBFF}, // FARSI YEH (ی)
	0x0620: {jtD, 0, 0, 0, 0},                      // (rare, keep isolated)

	// ── Lam-Alef ligatures (handled specially in reshape) ──────────
	// 0xFEDF (LAM initial) + 0xFE8E (ALEF final) etc. are composed
	// into U+FEF5–U+FEF8 (lam-alef ligatures) — see lamAlef table.
}

// lamAlef maps (lam+alef variant) → ligature codepoints.
// Key is the alef variant base rune; value = [isolated, final].
var lamAlef = map[rune][2]rune{
	0x0622: {0xFEF5, 0xFEF6}, // LAM-ALEF MADDA
	0x0623: {0xFEF7, 0xFEF8}, // LAM-ALEF HAMZA ABOVE
	0x0625: {0xFEF9, 0xFEFA}, // LAM-ALEF HAMZA BELOW
	0x0627: {0xFEFB, 0xFEFC}, // LAM-ALEF
}

// transparentSet: combining marks and control chars that don't break
// joining but are skipped when looking at neighbours.
var transparentSet = map[rune]bool{
	0x064B: true, 0x064C: true, 0x064D: true, 0x064E: true,
	0x064F: true, 0x0650: true, 0x0651: true, 0x0652: true,
	0x0653: true, 0x0654: true, 0x0655: true, 0x0670: true,
	0x0640: true, // TATWEEL/KASHIDA — transparent for joining logic
	0x200C: true, // ZWNJ
	0x200D: true, // ZWJ
	0x200E: true, 0x200F: true, // LRM/RLM
	0x0610: true, 0x0611: true, 0x0612: true, 0x0613: true,
	0x0614: true, 0x0615: true, 0x0616: true, 0x0617: true,
	0x0618: true, 0x0619: true, 0x061A: true,
}

func isTransparent(r rune) bool {
	return transparentSet[r]
}

func joiningTypeOf(r rune) joiningType {
	if isTransparent(r) {
		return jtT
	}
	if ac, ok := arMap[r]; ok {
		return ac.jt
	}
	return jtU
}

// canJoinLeft reports whether r can join with a preceding (left) letter,
// i.e. it accepts a join on its left side (types D or R or C).
func canJoinLeft(r rune) bool {
	jt := joiningTypeOf(r)
	return jt == jtD || jt == jtR || jt == jtC
}

// canJoinRight reports whether r can join with a following (right) letter,
// i.e. it accepts a join on its right side (types D or L or C).
func canJoinRight(r rune) bool {
	jt := joiningTypeOf(r)
	return jt == jtD || jt == jtL || jt == jtC
}

// pickForm returns the appropriate presentation-form rune for the
// letter at position i given whether it joins on the left and/or right.
func pickForm(ac arChar, joinLeft, joinRight bool) rune {
	switch {
	case joinLeft && joinRight:
		if ac.medial != 0 {
			return ac.medial
		}
		return ac.isolated
	case joinLeft && !joinRight:
		if ac.final != 0 {
			return ac.final
		}
		return ac.isolated
	case !joinLeft && joinRight:
		if ac.initial != 0 {
			return ac.initial
		}
		return ac.isolated
	default:
		return ac.isolated
	}
}
