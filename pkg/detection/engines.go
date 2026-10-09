package detection

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
)

// ── Unity ───────────────────────────────────────────────────────────

type UnityDetector struct{}

func (d *UnityDetector) Name() string  { return "unity" }
func (d *UnityDetector) Priority() int { return 100 }

func (d *UnityDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	res := &core.DetectionResult{
		Engine:   "unity",
		Metadata: make(map[string]string),
	}

	// Check for UnityPlayer.dll in root
	if scanner.FileExists(filepath.Join(gameDir, "UnityPlayer.dll")) {
		res.Confidence = 0.95
		res.Evidence = append(res.Evidence, "Found UnityPlayer.dll")
	} else if dataDir := scanner.FindDataDir(gameDir); dataDir != "" {
		if scanner.FileExists(filepath.Join(dataDir, "globalgamemanagers")) ||
			scanner.FileExists(filepath.Join(dataDir, "globalgamemanagers.assets")) {
			res.Confidence = 0.9
			res.DataPaths = append(res.DataPaths, dataDir)
			res.Evidence = append(res.Evidence, "Found globalgamemanagers in "+filepath.Base(dataDir))
		}
	} else if scanner.HasFileWithSuffix(gameDir, ".unity3d") {
		res.Confidence = 0.85
		res.Evidence = append(res.Evidence, "Found .unity3d bundle files")
	}

	if res.Confidence == 0 {
		return nil, nil // Not Unity
	}

	// Determine backend: Mono vs IL2CPP
	dataDir := scanner.FindDataDir(gameDir)
	if dataDir != "" && len(res.DataPaths) == 0 {
		res.DataPaths = append(res.DataPaths, dataDir)
	}

	if dataDir != "" {
		managedDir := filepath.Join(dataDir, "Managed")
		if scanner.DirExists(managedDir) {
			res.Backend = "mono"
			res.Evidence = append(res.Evidence, "Backend: Mono (Managed/ folder found)")
		}
	}

	// IL2CPP: GameAssembly.dll in root or data dir
	if scanner.FileExists(filepath.Join(gameDir, "GameAssembly.dll")) {
		res.Backend = "il2cpp"
		res.Evidence = append(res.Evidence, "Backend: IL2CPP (GameAssembly.dll found)")
	} else if dataDir != "" && scanner.FileExists(filepath.Join(dataDir, "GameAssembly.dll")) {
		res.Backend = "il2cpp"
		res.Evidence = append(res.Evidence, "Backend: IL2CPP")
	}

	// Try to extract version from globalgamemanagers
	if dataDir != "" {
		version := readUnityVersion(dataDir)
		if version != "" {
			res.Version = version
		}
	}

	exePath, gameName := findGameExeSkippingLaunchers(gameDir, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res, nil
}

func readUnityVersion(dataDir string) string {
	ggm := filepath.Join(dataDir, "globalgamemanagers")
	if !scanner.FileExists(ggm) {
		ggm = filepath.Join(dataDir, "globalgamemanagers.assets")
	}
	if !scanner.FileExists(ggm) {
		return ""
	}

	data, err := os.ReadFile(ggm)
	if err != nil || len(data) < 30 {
		return ""
	}

	content := string(data)
	for i := 0; i < len(content)-6; i++ {
		if (content[i] >= '2' && content[i] <= '2') && content[i+1] == '0' {
			end := i + 2
			for end < len(content) && (content[end] == '.' || (content[end] >= '0' && content[end] <= '9')) {
				end++
			}
			ver := content[i:end]
			if len(ver) >= 5 && strings.Count(ver, ".") >= 1 {
				parts := strings.Split(ver, ".")
				if len(parts) >= 2 {
					return ver
				}
			}
		}
	}

	if len(data) >= 20 {
		for _, hdrSize := range []int{0, 4, 8} {
			pos := hdrSize
			if pos+8 > len(data) {
				continue
			}
			pos += 8
			if pos+4 > len(data) {
				continue
			}
			strLen := int(binary.LittleEndian.Uint32(data[pos:]))
			pos += 4
			if strLen > 0 && strLen < 50 && pos+strLen <= len(data) {
				ver := strings.TrimRight(string(data[pos:pos+strLen]), "\x00")
				if strings.Count(ver, ".") >= 1 {
					return ver
				}
			}
		}
	}

	return ""
}

// ── Unreal Engine ───────────────────────────────────────────────────

type UnrealDetector struct{}

func (d *UnrealDetector) Name() string  { return "unreal" }
func (d *UnrealDetector) Priority() int { return 90 }

func (d *UnrealDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	pakFiles := scanner.WalkDir(gameDir, 3, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".pak")
	})

	hasContentDir := scanner.FindSubdir(gameDir, "Content") != "" ||
		scanner.WalkDir(gameDir, 3, func(p string) bool {
			return filepath.Base(p) == "Content"
		}) != nil

	hasShippingExe := scanner.WalkDir(gameDir, 3, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), "-win64-shipping.exe")
	}) != nil

	hasUasset := scanner.WalkDir(gameDir, 3, func(p string) bool {
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".uasset" || ext == ".umap" || ext == ".uexp"
	}) != nil

	hasEngineDir := scanner.FindSubdir(gameDir, "Engine") != ""

	score := 0
	res := &core.DetectionResult{
		Engine:   "unreal",
		Metadata: make(map[string]string),
	}

	if len(pakFiles) > 0 {
		score += 3
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .pak files", len(pakFiles)))
	}
	if hasShippingExe {
		score += 2
	}
	if hasUasset {
		score += 2
	}
	if hasContentDir {
		score += 1
	}
	if hasEngineDir {
		score += 1
	}

	if score >= 3 {
		res.Confidence = float64(score) / 7.0
		if res.Confidence > 1.0 {
			res.Confidence = 1.0
		}

		res.Version = detectUnrealVersion(gameDir, pakFiles)

		locresFiles := scanner.WalkDir(gameDir, 5, func(p string) bool {
			return strings.HasSuffix(strings.ToLower(p), ".locres")
		})
		if len(locresFiles) > 0 {
			res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .locres files", len(locresFiles)))
			res.DataPaths = append(res.DataPaths, filepath.Dir(locresFiles[0]))
		}

		exePath, gameName := findGameExeSkippingLaunchers(gameDir, []string{"-Win64-Shipping.exe", ".exe"})
		res.GameExe = exePath
		res.GameName = gameName

		return res, nil
	}

	return nil, nil
}

func detectUnrealVersion(root string, pakFiles []string) string {
	ue5Markers := []string{"Nanite", "Lumen", "VirtualShadowMaps"}
	ue4Markers := []string{"4.27", "4.26", "4.25", "4.24", "4.23", "4.22", "4.21", "4.20"}

	for _, marker := range ue5Markers {
		if scanner.WalkDir(root, 4, func(p string) bool {
			return strings.Contains(strings.ToLower(p), strings.ToLower(marker))
		}) != nil {
			return "UE5"
		}
	}

	configDir := scanner.FindSubdir(root, "Config")
	if configDir != "" {
		defaultEngine := filepath.Join(configDir, "DefaultEngine.ini")
		if scanner.FileExists(defaultEngine) {
			data, err := os.ReadFile(defaultEngine)
			if err == nil {
				content := strings.ToLower(string(data))
				for _, m := range ue4Markers {
					if strings.Contains(content, m) {
						return "UE4 " + m
					}
				}
			}
		}
	}

	if len(pakFiles) > 0 {
		return "UE4/UE5"
	}

	return ""
}

// ── Unreal Engine 3 ─────────────────────────────────────────────────

// UE3Detector detects Unreal Engine 3 games using markers that are
// distinct from UE4/UE5:
//   - Coalesced.* files (localization config bundles, e.g. Coalesced.INT)
//   - .upk files (UE3 package files)
//   - .xxx files (MK-specific compressed asset extension)
//
// UE4/UE5 games use .pak/.uasset/.umap and .locres, none of which are UE3
// markers, so this detector will not false-positive on them.
type UE3Detector struct{}

func (d *UE3Detector) Name() string  { return "ue3" }
func (d *UE3Detector) Priority() int { return 95 } // higher than Unreal (90)

func (d *UE3Detector) Detect(gameDir string) (*core.DetectionResult, error) {
	// Coalesced.* files (e.g. Coalesced.INT, Coalesced.ENG, Coalesced.ini)
	coalescedFiles := scanner.WalkDir(gameDir, 4, func(p string) bool {
		base := strings.ToLower(filepath.Base(p))
		return strings.HasPrefix(base, "coalesced.")
	})

	// .upk files (UE3 Unreal Package)
	upkFiles := scanner.WalkDir(gameDir, 4, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".upk")
	})

	// .xxx files (MK-specific compressed asset extension used by MK10)
	xxxFiles := scanner.WalkDir(gameDir, 4, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".xxx")
	})

	// Guard: if the game has .pak files (UE4/UE5 marker) and no UE3 markers,
	// it is NOT UE3 — bail out to avoid false positives.
	hasPak := scanner.WalkDir(gameDir, 3, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".pak")
	}) != nil
	if hasPak && len(coalescedFiles) == 0 && len(upkFiles) == 0 {
		return nil, nil
	}

	score := 0
	res := &core.DetectionResult{
		Engine:   "ue3",
		Metadata: make(map[string]string),
	}

	if len(coalescedFiles) > 0 {
		score += 3
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d Coalesced.* files", len(coalescedFiles)))
	}
	if len(upkFiles) > 0 {
		score += 2
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .upk files", len(upkFiles)))
	}
	if len(xxxFiles) > 0 {
		score += 2
		res.Evidence = append(res.Evidence, fmt.Sprintf("Found %d .xxx files (MK-style assets)", len(xxxFiles)))
	}

	// Need at least one strong UE3 marker
	if score < 2 {
		return nil, nil
	}

	res.Confidence = float64(score) / 7.0
	if res.Confidence > 1.0 {
		res.Confidence = 1.0
	}
	res.Version = "UE3"

	// UE3 does NOT use .locres (that's UE4/UE5)
	locresFiles := scanner.WalkDir(gameDir, 5, func(p string) bool {
		return strings.EqualFold(filepath.Ext(p), ".locres")
	})
	if len(locresFiles) > 0 {
		// If .locres exists alongside UE3 markers, note it but don't change engine
		res.Evidence = append(res.Evidence, fmt.Sprintf("Note: %d .locres files also present (may be hybrid)", len(locresFiles)))
	}

	exePath, gameName := findGameExeSkippingLaunchers(gameDir, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res, nil
}

// ── Godot ───────────────────────────────────────────────────────────

type GodotDetector struct{}

func (d *GodotDetector) Name() string  { return "godot" }
func (d *GodotDetector) Priority() int { return 90 }

func (d *GodotDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	pckFiles := scanner.WalkDir(gameDir, 2, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".pck")
	})

	hasGodotFile := scanner.WalkDir(gameDir, 1, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".godot")
	}) != nil

	hasProjectGodot := scanner.FileExists(filepath.Join(gameDir, "project.godot"))

	if len(pckFiles) > 0 || hasGodotFile || hasProjectGodot {
		res := &core.DetectionResult{
			Engine:     "godot",
			Confidence: 0.9,
			Evidence:   []string{fmt.Sprintf("Found %d .pck files", len(pckFiles))},
			Metadata:   make(map[string]string),
		}

		if hasProjectGodot {
			data, err := os.ReadFile(filepath.Join(gameDir, "project.godot"))
			if err == nil {
				content := string(data)
				if strings.Contains(content, "config_version=5") {
					res.Version = "4.x"
				} else if strings.Contains(content, "config_version=4") {
					res.Version = "3.x"
				}
			}
		}

		if res.Version == "" && len(pckFiles) > 0 {
			// Read the PCK and engine versions from the read-only header.
			header, err := readPCKHeader(pckFiles[0])
			if err == nil && header.Magic == "GDPC" {
				res.Version = pckEngineVersionToString(header)
				res.Evidence = append(res.Evidence,
					fmt.Sprintf("PCK header verified: magic=%s pack_version=%d engine_version=%s", header.Magic, header.PackVersion, res.Version))
				res.Metadata["pck_magic"] = header.Magic
				res.Metadata["pck_pack_version"] = fmt.Sprintf("%d", header.PackVersion)
				if header.EngineMajor > 0 {
					res.Metadata["pck_engine_version"] = fmt.Sprintf("%d.%d.%d", header.EngineMajor, header.EngineMinor, header.EnginePatch)
				}
			}
		}

		exePath, gameName := findGameExeSkippingLaunchers(gameDir, []string{".exe"})
		res.GameExe = exePath
		res.GameName = gameName

		return res, nil
	}

	return nil, nil
}

// readPCKVersion reads only the magic + version bytes from a .pck header
// (read-only, never writes). PCK format:
//
//	[0:4]  magic  = "GDPC"
//	[4:8]  pack version (uint32 LE)
//
// Version mapping:
//
//	v1 → Godot 2.x
//	v2 → Godot 3.x & 4.0–4.2
//	v3 → Godot 4.3+
func readPCKVersion(pckPath string) string {
	header, err := readPCKHeader(pckPath)
	if err != nil || header.Magic != "GDPC" {
		return ""
	}
	return pckEngineVersionToString(header)
}

// pckVersionToString maps a PCK pack-version integer to a human-readable
// Godot version string.
func pckVersionToString(packVersion uint32) string {
	switch packVersion {
	case 1:
		return "2.x"
	case 2:
		return "3.x / 4.0-4.2"
	case 3:
		return "4.3+"
	default:
		return fmt.Sprintf("pack_ver=%d", packVersion)
	}
}

type pckHeader struct {
	Magic       string
	PackVersion uint32
	EngineMajor uint32
	EngineMinor uint32
	EnginePatch uint32
}

func pckEngineVersionToString(header pckHeader) string {
	if header.EngineMajor > 0 {
		return fmt.Sprintf("%d.%d.%d", header.EngineMajor, header.EngineMinor, header.EnginePatch)
	}
	return pckVersionToString(header.PackVersion)
}

// readPCKHeader reads the PCK format and engine version fields read-only.
// PCK v1 only has magic and pack version; v2+ also stores engine major,
// minor, and patch versions.
func readPCKHeader(pckPath string) (pckHeader, error) {
	f, err := os.Open(pckPath)
	if err != nil {
		return pckHeader{}, err
	}
	defer f.Close()

	var hdr [20]byte
	if _, err := io.ReadFull(f, hdr[:8]); err != nil {
		return pckHeader{}, fmt.Errorf("read PCK magic and pack version: %w", err)
	}
	header := pckHeader{
		Magic:       string(hdr[:4]),
		PackVersion: binary.LittleEndian.Uint32(hdr[4:8]),
	}
	if header.PackVersion >= 2 {
		if _, err := io.ReadFull(f, hdr[8:20]); err != nil {
			return pckHeader{}, fmt.Errorf("read PCK engine version: %w", err)
		}
		header.EngineMajor = binary.LittleEndian.Uint32(hdr[8:12])
		header.EngineMinor = binary.LittleEndian.Uint32(hdr[12:16])
		header.EnginePatch = binary.LittleEndian.Uint32(hdr[16:20])
	}
	return header, nil
}

// ── Generic ─────────────────────────────────────────────────────────

// GenericDetector acts as a fallback for standard files
type GenericDetector struct{}

func (d *GenericDetector) Name() string  { return "generic" }
func (d *GenericDetector) Priority() int { return 0 }

func (d *GenericDetector) Detect(gameDir string) (*core.DetectionResult, error) {
	// Fallback implementation, very low confidence
	res := &core.DetectionResult{
		Engine:     "custom",
		Confidence: 0.1,
		Evidence:   []string{"No specific engine matched, falling back to generic."},
		Metadata:   make(map[string]string),
	}

	exePath, gameName := findGameExeSkippingLaunchers(gameDir, []string{".exe"})
	res.GameExe = exePath
	res.GameName = gameName

	return res, nil
}
