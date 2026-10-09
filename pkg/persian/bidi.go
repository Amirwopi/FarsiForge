package persian

import "golang.org/x/text/unicode/bidi"

// BidiReorder converts a logical-order string into visual-order
// suitable for engines that don't implement the Unicode BiDi algorithm.
// The base direction is set to RTL (Right-To-Left) which is correct
// for Persian text.
//
// After reshaping (which produces presentation-form glyphs in logical
// order), this function reverses the rune sequence so that when an
// LTR-only rendering engine draws the glyphs left-to-right, the text
// appears correctly right-to-left on screen.
func BidiReorder(s string) string {
	if s == "" {
		return s
	}

	// Use the full Unicode BiDi algorithm via golang.org/x/text.
	p := &bidi.Paragraph{}
	_, err := p.SetString(s, bidi.DefaultDirection(bidi.RightToLeft))
	if err != nil {
		// Fallback to simple reversal
		return SimpleVisualReorder(s)
	}

	ordering, err := p.Order()
	if err != nil {
		return SimpleVisualReorder(s)
	}

	var result []rune
	for i := 0; i < ordering.NumRuns(); i++ {
		run := ordering.Run(i)
		result = append(result, []rune(string(run.Bytes()))...)
	}

	return string(result)
}

// SimpleVisualReorder is a fallback BiDi reordering that handles the
// common case of single-line Persian text with embedded LTR segments
// (numbers, Latin words). It splits the text into directional runs and
// reverses RTL runs while keeping LTR runs in order.
func SimpleVisualReorder(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	var result []rune

	i := 0
	for i < len(runes) {
		isRTL := isRTLChar(runes[i])
		j := i
		for j < len(runes) && isRTLChar(runes[j]) == isRTL {
			j++
		}
		run := runes[i:j]
		if isRTL {
			reverseRunes(run)
		}
		result = append(result, run...)
		i = j
	}

	return string(result)
}

func isRTLChar(r rune) bool {
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
	return false
}

func reverseRunes(r []rune) {
	n := len(r)
	for i := 0; i < n/2; i++ {
		r[i], r[n-1-i] = r[n-1-i], r[i]
	}
}
