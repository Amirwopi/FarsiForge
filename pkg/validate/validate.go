// Package validate provides translation string validation.
//
// Ensures placeholders, tags, and structure remain intact after translation
// to prevent game crashes or UI breakage.
package validate

import (
	"regexp"
	"strings"

	"farsiforge/pkg/core"
)

// StandardValidator implements core.IValidator.
type StandardValidator struct {
	// Patterns for placeholders
	Placeholders []*regexp.Regexp
}

// New creates a new StandardValidator with common placeholder patterns.
func New() *StandardValidator {
	return &StandardValidator{
		Placeholders: []*regexp.Regexp{
			regexp.MustCompile(`{[a-zA-Z0-9_]+}`),           // {player}, {0}
			regexp.MustCompile(`%[dsfpxX]`),                 // printf style
			regexp.MustCompile(`\n`),                        // newlines
			regexp.MustCompile(`\r`),                        // carriage returns
			regexp.MustCompile(`<[^>]+>`),                   // XML/HTML tags (Unity Rich Text)
			regexp.MustCompile(`\[[^\]]+\]`),                // BBCode tags
			regexp.MustCompile(`\\[a-z]`),                   // Escapes like \n, \t
		},
	}
}

// Validate checks all entries and returns a list of validation errors.
func (v *StandardValidator) Validate(entries []core.StringEntry) []core.ValidationError {
	var errs []core.ValidationError

	for _, entry := range entries {
		// Only validate translated strings
		if entry.Status != core.StatusTranslated && entry.Status != core.StatusApproved {
			continue
		}
		if entry.Translation == "" {
			continue
		}

		// 1. Placeholder validation
		for _, pattern := range v.Placeholders {
			srcMatches := extractMatches(entry.Source, pattern)
			trMatches := extractMatches(entry.Translation, pattern)
			
			if !compareMatches(srcMatches, trMatches) {
				errs = append(errs, core.ValidationError{
					EntryID:  entry.ID,
					Type:     core.ValPlaceholderMismatch,
					Message:  "Placeholders/tags do not match original string",
					Severity: core.SevError,
					Details:  fmtMatches(srcMatches, trMatches),
				})
			}
		}

		// 2. Length validation
		if entry.MaxLength > 0 && len([]rune(entry.Translation)) > entry.MaxLength {
			errs = append(errs, core.ValidationError{
				EntryID:  entry.ID,
				Type:     core.ValLengthExceeded,
				Message:  "Translation exceeds maximum length",
				Severity: core.SevWarning,
			})
		}
	}

	return errs
}

func extractMatches(s string, pattern *regexp.Regexp) map[string]int {
	matches := pattern.FindAllString(s, -1)
	counts := make(map[string]int)
	for _, m := range matches {
		counts[m]++
	}
	return counts
}

func compareMatches(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, count := range a {
		if b[k] != count {
			return false
		}
	}
	return true
}

func fmtMatches(src, tr map[string]int) string {
	var parts []string
	parts = append(parts, "Source:")
	for k, c := range src {
		parts = append(parts, k+"(x"+string(rune(c+'0'))+")")
	}
	if len(src) == 0 {
		parts = append(parts, "none")
	}
	
	parts = append(parts, "| Translated:")
	for k, c := range tr {
		parts = append(parts, k+"(x"+string(rune(c+'0'))+")")
	}
	if len(tr) == 0 {
		parts = append(parts, "none")
	}
	
	return strings.Join(parts, " ")
}
