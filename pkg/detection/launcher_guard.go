package detection

import (
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/scanner"
)

// isLauncherExe returns true if the executable filename looks like a
// launcher (case-insensitive). These should be skipped when looking for
// the actual game executable.
func isLauncherExe(name string) bool {
	return strings.Contains(strings.ToLower(name), "launcher")
}

// findGameExeSkippingLaunchers wraps scanner.FindGameExe but filters out
// any executable whose filename contains "launcher" (case-insensitive).
// It falls back to scanner.FindGameExe if the filtered search finds nothing.
func findGameExeSkippingLaunchers(gameRoot string, patterns []string) (exePath, gameName string) {
	entries, err := os.ReadDir(gameRoot)
	if err != nil {
		return scanner.FindGameExe(gameRoot, patterns)
	}

	for _, pat := range patterns {
		lowerPat := strings.ToLower(pat)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), lowerPat) {
				continue
			}
			lower := strings.ToLower(name)
			if strings.Contains(lower, "uninstall") ||
				strings.Contains(lower, "crash") ||
				strings.Contains(lower, "redist") ||
				strings.Contains(lower, "setup") ||
				isLauncherExe(name) {
				continue
			}
			return filepath.Join(gameRoot, name), strings.TrimSuffix(name, filepath.Ext(name))
		}
	}

	// Fallback to the original scanner implementation (which does not
	// skip launchers) so we never return empty if a launcher is the only exe.
	return scanner.FindGameExe(gameRoot, patterns)
}
