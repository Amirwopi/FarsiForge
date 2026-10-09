package detection

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
)

// ── Nested-dir root resolver ────────────────────────────────────────
//
// Several games ship with the real game root nested 1–2 levels below the
// directory the user points at (e.g. ELDEN RING\Game, Factorio\Factorio,
// Project.Zomboid\Project Zomboid). resolveRoots returns candidate roots in
// order — the input dir first, then immediate subdirectories, then their
// subdirectories — skipping redistributable/support dirs that never contain
// game markers. Each detector probes these roots in order and records the
// first match in Metadata["resolved_root"].
//
// IMPORTANT: probes must check markers at the ROOT level (or in known
// subdirectories like Data\ or bin\x64\), NOT recurse arbitrarily. Otherwise
// a probe on the parent dir would match markers that actually live in a
// nested game root, and resolved_root would point at the wrong directory.

var skipRootDirs = map[string]bool{
	"_commonredist":  true,
	"directx":        true,
	"redist":         true,
	"__installer":    true,
	"support":        true,
	"manuals":        true,
	"trailers":       true,
	".egstore":       true,
	"cloud":          true,
	"movies":         true,
	"moviesxml":      true,
	"mss":            true,
	"steam_settings": true,
}

func resolveRoots(gameDir string) []string {
	roots := []string{gameDir}

	entries, err := os.ReadDir(gameDir)
	if err != nil {
		return roots
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if skipRootDirs[strings.ToLower(e.Name())] {
			continue
		}
		sub := filepath.Join(gameDir, e.Name())
		roots = append(roots, sub)

		// Depth 2: subdirectories of the immediate subdirectory.
		subEntries, err := os.ReadDir(sub)
		if err != nil {
			continue
		}
		for _, e2 := range subEntries {
			if !e2.IsDir() {
				continue
			}
			if skipRootDirs[strings.ToLower(e2.Name())] {
				continue
			}
			roots = append(roots, filepath.Join(sub, e2.Name()))
		}
	}

	return roots
}

// ── SAGE (Command & Conquer Generals / Zero Hour / Red Alert 3) ─────

type SAGEDetector struct{}

func (d *SAGEDetector) Name() string  { return "sage" }
func (d *SAGEDetector) Priority() int { return 95 }

func (d *SAGEDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *SAGEDetector) probe(root string) *core.DetectionResult {
	// .SkuDef files at root (Red Alert 3 SKU definition files)
	skuDefFiles := scanner.WalkDir(root, 0, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".skudef")
	})

	// Data\generals.exe (Generals / Zero Hour)
	hasGenerals := scanner.FileExists(filepath.Join(root, "Data", "generals.exe"))

	// .big archive files in Data\ (SAGE archive format)
	bigFiles := scanner.WalkDir(filepath.Join(root, "Data"), 0, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".big")
	})

	// RA3.exe at root (Red Alert 3)
	hasRA3 := scanner.FileExists(filepath.Join(root, "RA3.exe"))

	// BINKW32.DLL in Data\ (Bink video, shipped with SAGE games)
	hasBink := scanner.FileExists(filepath.Join(root, "Data", "BINKW32.DLL"))

	score := 0
	res := &core.DetectionResult{
		Engine:   "sage",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if len(skuDefFiles) > 0 {
		score += 4
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .SkuDef files", len(skuDefFiles)))
	}
	if hasGenerals {
		score += 4
		res.Evidence = append(res.Evidence, "Found Data/generals.exe")
	}
	if len(bigFiles) > 0 {
		score += 2
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .big files", len(bigFiles)))
	}
	if hasRA3 {
		score += 2
		res.Evidence = append(res.Evidence, "Found RA3.exe")
	}
	if hasBink {
		score += 1
		res.Evidence = append(res.Evidence, "Found Data/BINKW32.DLL")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 7.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if len(bigFiles) > 0 {
		res.DataPaths = append(res.DataPaths, filepath.Dir(bigFiles[0]))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── GoldSrc (Half-Life / Counter-Strike 1.6) ────────────────────────

type GoldSrcDetector struct{}

func (d *GoldSrcDetector) Name() string  { return "goldsrc" }
func (d *GoldSrcDetector) Priority() int { return 95 }

func (d *GoldSrcDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *GoldSrcDetector) probe(root string) *core.DetectionResult {
	hasHL := scanner.FileExists(filepath.Join(root, "hl.exe"))
	hasValve := scanner.DirExists(filepath.Join(root, "valve"))
	hasHW := scanner.FileExists(filepath.Join(root, "hw.dll"))
	hasSW := scanner.FileExists(filepath.Join(root, "sw.dll"))
	hasFS := scanner.FileExists(filepath.Join(root, "FileSystem_Stdio.dll"))
	hasTier0 := scanner.FileExists(filepath.Join(root, "tier0_s.dll"))

	score := 0
	res := &core.DetectionResult{
		Engine:   "goldsrc",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if hasHL {
		score += 3
		res.Evidence = append(res.Evidence, "Found hl.exe")
	}
	if hasValve {
		score += 2
		res.Evidence = append(res.Evidence, "Found valve/ directory")
	}
	if hasHW || hasSW {
		score += 2
		res.Evidence = append(res.Evidence, "Found hw.dll/sw.dll (GoldSrc engine DLLs)")
	}
	if hasFS {
		score += 1
		res.Evidence = append(res.Evidence, "Found FileSystem_Stdio.dll")
	}
	if hasTier0 {
		score += 1
		res.Evidence = append(res.Evidence, "Found tier0_s.dll")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 9.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if hasValve {
		res.DataPaths = append(res.DataPaths, filepath.Join(root, "valve"))
	}
	if scanner.DirExists(filepath.Join(root, "cstrike")) {
		res.DataPaths = append(res.DataPaths, filepath.Join(root, "cstrike"))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── Valve Source (Source 1) ────────────────────────────────────────

type SourceDetector struct{}

func (d *SourceDetector) Name() string  { return "source" }
func (d *SourceDetector) Priority() int { return 95 }

func (d *SourceDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *SourceDetector) probe(root string) *core.DetectionResult {
	// gameinfo.txt (Source 1) — NOT gameinfo.gi (that's Source 2)
	hasGameinfo := scanner.FileExists(filepath.Join(root, "gameinfo.txt")) ||
		scanner.WalkDir(root, 2, func(p string) bool {
			return strings.EqualFold(filepath.Base(p), "gameinfo.txt")
		}) != nil

	// .vpk files (Valve package)
	vpkFiles := scanner.WalkDir(root, 2, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".vpk")
	})

	// Classic Source engine exes
	hasExe := scanner.WalkDir(root, 1, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return base == "hl2.exe" || base == "left4dead2.exe" || base == "portal2.exe" ||
			base == "csgo.exe" || base == "garrysmod.exe" || base == "tf2.exe" ||
			base == "swarm.exe" || base == "dota2.exe"
	}) != nil

	score := 0
	res := &core.DetectionResult{
		Engine:   "source",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if hasGameinfo {
		score += 4
		res.Evidence = append(res.Evidence, "Found gameinfo.txt")
	}
	if len(vpkFiles) > 0 {
		score += 3
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .vpk files", len(vpkFiles)))
	}
	if hasExe {
		score += 2
		res.Evidence = append(res.Evidence, "Found Source engine executable")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 9.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if len(vpkFiles) > 0 {
		res.DataPaths = append(res.DataPaths, filepath.Dir(vpkFiles[0]))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── Valve Source 2 ──────────────────────────────────────────────────

type Source2Detector struct{}

func (d *Source2Detector) Name() string  { return "source2" }
func (d *Source2Detector) Priority() int { return 95 }

func (d *Source2Detector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *Source2Detector) probe(root string) *core.DetectionResult {
	// gameinfo.gi (Source 2)
	hasGameinfo := scanner.FileExists(filepath.Join(root, "gameinfo.gi")) ||
		scanner.WalkDir(root, 3, func(p string) bool {
			return strings.EqualFold(filepath.Base(p), "gameinfo.gi")
		}) != nil

	// game/ top-level dir with bin\win64 subdir (CS2 / Deadlock layout)
	hasGameDir := scanner.DirExists(filepath.Join(root, "game"))
	hasBinWin64 := scanner.DirExists(filepath.Join(root, "game", "bin", "win64"))

	// .vpk files (CS2 ships pak01_dir.vpk etc.)
	vpkFiles := scanner.WalkDir(root, 3, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".vpk")
	})

	// Source 2 game executables
	hasExe := scanner.WalkDir(root, 2, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return base == "cs2.exe" || base == "deadlock.exe" || base == "dota2.exe"
	}) != nil

	score := 0
	res := &core.DetectionResult{
		Engine:   "source2",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if hasGameinfo {
		score += 4
		res.Evidence = append(res.Evidence, "Found gameinfo.gi")
	}
	if hasGameDir && hasBinWin64 {
		score += 2
		res.Evidence = append(res.Evidence, "Found game/bin/win64/ layout")
	}
	if len(vpkFiles) > 0 {
		score += 2
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .vpk files", len(vpkFiles)))
	}
	if hasExe {
		score += 2
		res.Evidence = append(res.Evidence, "Found Source 2 executable")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 10.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── Rockstar RAGE (GTA V) ───────────────────────────────────────────

type RAGEDetector struct{}

func (d *RAGEDetector) Name() string  { return "rage" }
func (d *RAGEDetector) Priority() int { return 95 }

func (d *RAGEDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *RAGEDetector) probe(root string) *core.DetectionResult {
	// .rpf archives (RAGE package format)
	rpfFiles := scanner.WalkDir(root, 2, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".rpf")
	})

	// GTA5.exe / RDR2.exe
	hasExe := scanner.FileExists(filepath.Join(root, "GTA5.exe")) ||
		scanner.FileExists(filepath.Join(root, "RDR2.exe"))

	// update/ + x64/ dirs (GTA V layout)
	hasUpdate := scanner.DirExists(filepath.Join(root, "update"))
	hasX64 := scanner.DirExists(filepath.Join(root, "x64"))

	score := 0
	res := &core.DetectionResult{
		Engine:   "rage",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if len(rpfFiles) > 0 {
		score += 4
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .rpf files", len(rpfFiles)))
	}
	if hasExe {
		score += 3
		res.Evidence = append(res.Evidence, "Found RAGE game executable")
	}
	if hasUpdate && hasX64 {
		score += 2
		res.Evidence = append(res.Evidence, "Found update/ + x64/ layout")
	}

	if score < 5 {
		return nil
	}

	res.Confidence = float64(score) / 9.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if len(rpfFiles) > 0 {
		res.DataPaths = append(res.DataPaths, filepath.Dir(rpfFiles[0]))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── FromSoftware (Dark Souls / ELDEN RING) ──────────────────────────

type FromSoftwareDetector struct{}

func (d *FromSoftwareDetector) Name() string  { return "fromsoftware" }
func (d *FromSoftwareDetector) Priority() int { return 95 }

func (d *FromSoftwareDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *FromSoftwareDetector) probe(root string) *core.DetectionResult {
	// .bdt/.bhd archive pairs at root (FromSoftware archive format)
	bdtFiles := scanner.WalkDir(root, 0, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".bdt")
	})
	bhdFiles := scanner.WalkDir(root, 0, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".bhd")
	})

	// regulation.bin at root (Dark Souls / ELDEN RING parameter file)
	hasRegulation := scanner.FileExists(filepath.Join(root, "regulation.bin"))

	// eldenring.exe at root
	hasEldenRing := scanner.FileExists(filepath.Join(root, "eldenring.exe"))

	// sd/ directory (streaming data)
	hasSD := scanner.DirExists(filepath.Join(root, "sd"))

	score := 0
	res := &core.DetectionResult{
		Engine:   "fromsoftware",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if len(bdtFiles) > 0 {
		score += 3
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .bdt files", len(bdtFiles)))
	}
	if len(bhdFiles) > 0 {
		score += 2
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .bhd files", len(bhdFiles)))
	}
	if hasRegulation {
		score += 2
		res.Evidence = append(res.Evidence, "Found regulation.bin")
	}
	if hasEldenRing {
		score += 2
		res.Evidence = append(res.Evidence, "Found eldenring.exe")
	}
	if hasSD {
		score += 1
		res.Evidence = append(res.Evidence, "Found sd/ directory")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 10.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if len(bdtFiles) > 0 {
		res.DataPaths = append(res.DataPaths, filepath.Dir(bdtFiles[0]))
	}
	if hasSD {
		res.DataPaths = append(res.DataPaths, filepath.Join(root, "sd"))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── Factorio ────────────────────────────────────────────────────────

type FactorioDetector struct{}

func (d *FactorioDetector) Name() string  { return "factorio" }
func (d *FactorioDetector) Priority() int { return 95 }

func (d *FactorioDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *FactorioDetector) probe(root string) *core.DetectionResult {
	hasConfigPath := scanner.FileExists(filepath.Join(root, "config-path.cfg"))
	hasFactorioExe := scanner.FileExists(filepath.Join(root, "bin", "x64", "factorio.exe"))
	hasDataCore := scanner.DirExists(filepath.Join(root, "data", "core"))
	hasBinX64 := scanner.DirExists(filepath.Join(root, "bin", "x64"))

	score := 0
	res := &core.DetectionResult{
		Engine:   "factorio",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if hasConfigPath {
		score += 4
		res.Evidence = append(res.Evidence, "Found config-path.cfg")
	}
	if hasFactorioExe {
		score += 3
		res.Evidence = append(res.Evidence, "Found bin/x64/factorio.exe")
	}
	if hasDataCore {
		score += 2
		res.Evidence = append(res.Evidence, "Found data/core/ directory")
	}
	if hasBinX64 {
		score += 1
		res.Evidence = append(res.Evidence, "Found bin/x64/ directory")
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 10.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if hasDataCore {
		res.DataPaths = append(res.DataPaths, filepath.Join(root, "data"))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}

// ── Project Zomboid ─────────────────────────────────────────────────

type ZomboidDetector struct{}

func (d *ZomboidDetector) Name() string  { return "zomboid" }
func (d *ZomboidDetector) Priority() int { return 95 }

func (d *ZomboidDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	for _, root := range resolveRoots(gameDir) {
		if res := d.probe(root); res != nil {
			res.Metadata["resolved_root"] = root
			return res, nil
		}
	}
	return nil, nil
}

func (d *ZomboidDetector) probe(root string) *core.DetectionResult {
	// ProjectZomboid*.exe at root
	pzExe := scanner.WalkDir(root, 0, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return strings.HasPrefix(base, "projectzomboid") && strings.HasSuffix(base, ".exe")
	})

	// zombie/ directory (Java package namespace, unique to Project Zomboid)
	hasZombieDir := scanner.DirExists(filepath.Join(root, "zombie"))

	// media/ directory (game data + lua translations)
	hasMedia := scanner.DirExists(filepath.Join(root, "media"))

	// Bundled JRE
	hasJRE := scanner.DirExists(filepath.Join(root, "jre64")) || scanner.DirExists(filepath.Join(root, "jre"))

	// .jar files at root (Java runtime)
	jarFiles := scanner.WalkDir(root, 0, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".jar")
	})

	score := 0
	res := &core.DetectionResult{
		Engine:   "zomboid",
		Backend:  "none",
		Metadata: make(map[string]string),
	}

	if len(pzExe) > 0 {
		score += 3
		res.Evidence = append(res.Evidence, "Found ProjectZomboid*.exe")
	}
	if hasZombieDir {
		score += 3
		res.Evidence = append(res.Evidence, "Found zombie/ directory")
	}
	if hasMedia {
		score += 1
		res.Evidence = append(res.Evidence, "Found media/ directory")
	}
	if hasJRE {
		score += 1
		res.Evidence = append(res.Evidence, "Found bundled JRE (jre/jre64)")
	}
	if len(jarFiles) > 0 {
		score += 1
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .jar files", len(jarFiles)))
	}

	if score < 4 {
		return nil
	}

	res.Confidence = float64(score) / 9.0
	if res.Confidence > 0.95 {
		res.Confidence = 0.95
	}

	if hasMedia {
		res.DataPaths = append(res.DataPaths, filepath.Join(root, "media"))
	}

	exePath, gameName := findGameExeSkippingLaunchers(root, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res
}
