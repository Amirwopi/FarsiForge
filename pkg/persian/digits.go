package persian

// Persian digits (۰-۹) and their ASCII equivalents.
const (
	Persian0 = '\u06F0'
	Persian1 = '\u06F1'
	Persian2 = '\u06F2'
	Persian3 = '\u06F3'
	Persian4 = '\u06F4'
	Persian5 = '\u06F5'
	Persian6 = '\u06F6'
	Persian7 = '\u06F7'
	Persian8 = '\u06F8'
	Persian9 = '\u06F9'
)

// Arabic digits (٠-٩) — sometimes used instead of Persian.
const (
	Arabic0 = '\u0660'
)

// ToPersianDigits converts ASCII digits (0-9) and Arabic-Indic digits
// (٠-٩) to Persian digits (۰-۹) in the given string.
func ToPersianDigits(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r >= '0' && r <= '9':
			runes[i] = Persian0 + (r - '0')
		case r >= Arabic0 && r <= Arabic0+9:
			runes[i] = Persian0 + (r - Arabic0)
		}
	}
	return string(runes)
}

// ToASCIIDigits converts Persian and Arabic-Indic digits back to ASCII.
func ToASCIIDigits(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case r >= Persian0 && r <= Persian9:
			runes[i] = '0' + (r - Persian0)
		case r >= Arabic0 && r <= Arabic0+9:
			runes[i] = '0' + (r - Arabic0)
		}
	}
	return string(runes)
}

// PersianPunctuation normalises common punctuation marks to forms
// that display correctly in RTL context within game engines.
// For example, converting "?" to "؟" and "," to "،".
func PersianPunctuation(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		switch r {
		case '?':
			runes[i] = '\u061F' // ؟
		case ',':
			runes[i] = '\u060C' // ،
		case ';':
			runes[i] = '\u061B' // ؛
		}
	}
	return string(runes)
}

// FixYeh replaces Arabic YEH (ي, U+064A) with Persian FARSI YEH (ی, U+06CC)
// and Arabic KAF (ك, U+0643) with Persian KEHEH (ک, U+06A9).
// This is important for correct Persian rendering.
func FixYeh(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		switch r {
		case 0x064A: // Arabic YEH
			runes[i] = 0x06CC // Persian FARSI YEH
		case 0x0643: // Arabic KAF
			runes[i] = 0x06A9 // Persian KEHEH
		case 0x0649: // ALEF MAKSURA
			runes[i] = 0x06CC // Persian FARSI YEH
		}
	}
	return string(runes)
}
