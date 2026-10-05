package persian

import (
	"strings"
)

// Reshape converts Arabic/Persian text from logical order with base
// (isolated) Unicode characters into presentation forms with correct
// joining. This is the core shaping algorithm.
//
// The algorithm:
//  1. Walk the string in logical order (left-to-right).
//  2. For each Arabic letter, determine if it joins on its left side
//     (previous letter can join right) and on its right side (next
//     letter can join left).
//  3. Select the appropriate presentation form (isolated/initial/
//     medial/final).
//  4. Handle special cases: lam-alef ligatures, ZWNJ (zero-width
//     non-joiner), transparent characters (combining marks).
func Reshape(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	n := len(runes)
	result := make([]rune, 0, n)

	for i := 0; i < n; i++ {
		r := runes[i]

		// Skip and pass through transparent characters (combining marks,
		// ZWNJ, ZWJ, etc.) — they attach to whatever form was already chosen.
		if isTransparent(r) {
			// ZWNJ (U+200C) breaks joining: we just drop it from output
			// because the presentation forms already encode the break.
			// ZWJ (U+200D) forces joining: handled by not breaking the chain.
			if r == 0x200C {
				continue // drop ZWNJ — it prevents joining, which is already
				// handled by the neighbour detection logic since ZWNJ is
				// transparent and doesn't satisfy canJoinLeft/canJoinRight.
			}
			result = append(result, r)
			continue
		}

		ac, ok := arMap[r]
		if !ok {
			// Non-Arabic character — pass through as-is.
			result = append(result, r)
			continue
		}

		// ── Lam-Alef ligature detection ──────────────────────────────
		// If this is LAM (jtD) and the next non-transparent character is
		// an ALEF variant, compose the ligature.
		if r == 0x0644 { // LAM
			nextR := nextNonTransparent(runes, i+1)
			if la, ok := lamAlef[nextR]; ok {
				// Determine if this ligature is in final position
				// (i.e., the character after the ALEF doesn't join left).
				afterAlef := nextNonTransparent(runes, i+2)
				joinLeft := canJoinLeft(afterAlef)
				if joinLeft {
					// medial — but lam-alef only has isolated/final forms
					result = append(result, la[0]) // isolated (rare)
				} else {
					result = append(result, la[1]) // final
				}
				// Skip the ALEF (and any transparents between LAM and ALEF)
				i = indexAfter(runes, i+1, nextR)
				continue
			}
		}

		// ── Determine joining for this character ─────────────────────
		prevR := prevNonTransparent(runes, i-1)
		nextR := nextNonTransparent(runes, i+1)

		joinLeft := canJoinRight(prevR) && canJoinLeft(r)
		joinRight := canJoinRight(r) && canJoinLeft(nextR)

		form := pickForm(ac, joinLeft, joinRight)
		result = append(result, form)
	}

	return string(result)
}

// nextNonTransparent returns the next non-transparent rune at or after
// position start, or 0 if none.
func nextNonTransparent(runes []rune, start int) rune {
	for i := start; i < len(runes); i++ {
		if !isTransparent(runes[i]) {
			return runes[i]
		}
	}
	return 0
}

// prevNonTransparent returns the previous non-transparent rune at or
// before position start, or 0 if none.
func prevNonTransparent(runes []rune, start int) rune {
	for i := start; i >= 0; i-- {
		if !isTransparent(runes[i]) {
			return runes[i]
		}
	}
	return 0
}

// indexAfter returns the index after the first occurrence of target
// at or after start (skipping transparents), or start if not found.
func indexAfter(runes []rune, start int, target rune) int {
	for i := start; i < len(runes); i++ {
		if !isTransparent(runes[i]) {
			if runes[i] == target {
				return i
			}
			return start // different non-transparent char, abort
		}
	}
	return start
}

// ── High-level processing ──────────────────────────────────────────

// Options controls how Persian text is processed for game injection.
type Options struct {
	Reshape       bool // Convert to presentation forms (default: true)
	BidiReorder   bool // Reorder for visual RTL display (default: true)
	FixYeh        bool // Replace Arabic Yeh/Kaf with Persian (default: true)
	PersianDigits bool // Convert digits to Persian (default: true)
	ConvertPunct  bool // Convert ? , ; to Persian equivalents (default: false)
	DropDiacritics bool // Remove Arabic diacritics/harakat (default: false)
}

// DefaultOptions returns the standard processing options used for
// most game localization scenarios.
func DefaultOptions() Options {
	return Options{
		Reshape:       true,
		BidiReorder:   true,
		FixYeh:        true,
		PersianDigits: true,
		ConvertPunct:  false,
		DropDiacritics: false,
	}
}

// Process applies the full Persian text processing pipeline to a
// translated string, preparing it for injection into a game that
// lacks native RTL/Arabic shaping support.
//
// The pipeline order:
//  1. Fix Yeh/Kaf (normalize Arabic → Persian letters)
//  2. Convert punctuation (optional)
//  3. Convert digits to Persian
//  4. Reshape to presentation forms
//  5. BiDi reorder for visual RTL
func Process(s string, opts Options) string {
	if s == "" {
		return s
	}

	out := s

	if opts.FixYeh {
		out = FixYeh(out)
	}

	if opts.ConvertPunct {
		out = PersianPunctuation(out)
	}

	if opts.PersianDigits {
		out = ToPersianDigits(out)
	}

	if opts.DropDiacritics {
		out = dropDiacritics(out)
	}

	if opts.Reshape {
		out = Reshape(out)
	}

	if opts.BidiReorder {
		out = BidiReorder(out)
	}

	return out
}

// ProcessDefault applies the standard processing pipeline.
func ProcessDefault(s string) string {
	return Process(s, DefaultOptions())
}

// dropDiacritics removes Arabic harakat (tashkil/diacritical marks).
func dropDiacritics(s string) string {
	runes := []rune(s)
	result := make([]rune, 0, len(runes))
	for _, r := range runes {
		if isTransparent(r) && r != 0x200D { // keep ZWJ
			continue
		}
		result = append(result, r)
	}
	return string(result)
}

// HasPersian reports whether the string contains any Persian/Arabic
// characters that would need shaping.
func HasPersian(s string) bool {
	for _, r := range s {
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
		if r >= 0x0750 && r <= 0x077F {
			return true
		}
		if r >= 0xFB50 && r <= 0xFDFF {
			return true
		}
		if r >= 0xFE70 && r <= 0xFEFF {
			return true
		}
	}
	return false
}

// IsProcessed reports whether a string already appears to be in
// presentation-form + visual-order (i.e., already processed).
func IsProcessed(s string) bool {
	for _, r := range s {
		if r >= 0xFE70 && r <= 0xFEFF {
			return true
		}
		if r >= 0xFB50 && r <= 0xFDFF {
			return true
		}
	}
	return false
}

// CleanText removes control characters and normalizes whitespace.
func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}
