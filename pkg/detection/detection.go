package detection

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Engine identifies a game engine.
type Engine string

const (
	EngineUnknown    Engine = "unknown"
	EngineUnity      Engine = "unity"
	EngineUnreal     Engine = "unreal"
	EngineGodot      Engine = "godot"
	EngineRPGMaker   Engine = "rpgmaker"
	EngineGameMaker  Engine = "gamemaker"
	EngineRenPy      Engine = "renpy"
	EngineSource     Engine = "source"
	EngineGoldSrc    Engine = "goldsrc"
	EngineAdobeAIR   Engine = "adobe_air"
	EngineCustom     Engine = "custom"
)

// Backend identifies the sub-technology (e.g. Unity Mono vs IL2CPP).
type Backend string

const (
	BackendUnknown Engine = ""
	BackendMono    Backend = "mono"
	BackendIL2CPP  Backend = "il2cpp"
)

// GameInfo holds detected engine information.
type GameInfo struct {
	Engine       Engine  `json:"engine"`
	Backend      Backend `json:"backend"`
	Version      string  `json:"version"`
	GameName     string  `json:"game_name"`
	GameExe      string  `json:"game_exe"`
	GameRoot     string  `json:"game_root"`
	DataPath     string  `json:"data_path"`
	Confidence   float64 `json:"confidence"`
	Notes        []string `json:"notes"`
}

// Detect scans a game directory and returns engine identification.
func Detect(gameDir string) (*GameInfo, error) {
	info := &GameInfo{
		GameRoot: gameDir,
	}

	absDir, err := filepath.Abs(gameDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path: %w", err)
	}
	info.GameRoot = absDir

	// Try each detector in order of specificity.
	detectors := []func(*GameInfo) bool{
		detectUnity,
		detectUnreal,
		detectGodot,
		detectRPGMaker,
		detectGameMaker,
		detectRenPy,
		detectAdobeAIR,
		detectGoldSrc,
		detectSource,
	}

	for _, det := range detectors {
		if det(info) {
			break
		}
	}

	if info.Engine == EngineUnknown {
		info.Engine = EngineCustom
		info.Notes = append(info.Notes, "No known engine detected — generic text file mode")
	}

	// Find the main executable
	findGameExe(info)

	return info, nil
}

// findGameExe looks for the main game executable in the root directory.
func findGameExe(info *GameInfo) {
	entries, err := os.ReadDir(info.GameRoot)
	if err != nil {
		return
	}

	// Priority patterns by engine
	var patterns []string
	switch info.Engine {
	case EngineUnity:
		patterns = []string{".exe"}
	case EngineUnreal:
		patterns = []string{"-Win64-Shipping.exe", ".exe"}
	case EngineGodot:
		patterns = []string{".exe"}
	case EngineRPGMaker:
		patterns = []string{"Game.exe", ".exe"}
	case EngineGameMaker:
		patterns = []string{".exe"}
	case EngineRenPy:
		patterns = []string{".exe"}
	default:
		patterns = []string{".exe"}
	}

	for _, pat := range patterns {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(pat)) {
				// Skip obvious non-game executables
				lower := strings.ToLower(name)
				if strings.Contains(lower, "uninstall") ||
					strings.Contains(lower, "crash") ||
					strings.Contains(lower, "redist") ||
					strings.Contains(lower, "setup") {
					continue
				}
				info.GameExe = filepath.Join(info.GameRoot, name)
				info.GameName = strings.TrimSuffix(name, filepath.Ext(name))
				return
			}
		}
	}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// hasFileWithSuffix checks if a directory contains a file with the given suffix.
func hasFileWithSuffix(dir, suffix string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}

// hasFileWithPrefix checks if a directory contains a file with the given prefix.
func hasFileWithPrefix(dir, prefix string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

// findSubdir finds a subdirectory with the given name (case-insensitive).
func findSubdir(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// findDataDir finds a *_Data directory (Unity convention).
func findDataDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), "_data") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// walkDir walks up to maxDepth levels and returns all file paths matching a predicate.
func walkDir(root string, maxDepth int, pred func(string) bool) []string {
	var results []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && pred(path) {
			results = append(results, path)
		}
		return nil
	})
	return results
}
