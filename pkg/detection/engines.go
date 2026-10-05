package detection

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── Unity ───────────────────────────────────────────────────────────

func detectUnity(info *GameInfo) bool {
	root := info.GameRoot

	// Check for UnityPlayer.dll in root
	if fileExists(filepath.Join(root, "UnityPlayer.dll")) {
		info.Engine = EngineUnity
		info.Confidence = 0.95
		info.Notes = append(info.Notes, "Found UnityPlayer.dll")
	} else if dataDir := findDataDir(root); dataDir != "" {
		// Check for globalgamemanagers in *_Data folder
		if fileExists(filepath.Join(dataDir, "globalgamemanagers")) ||
			fileExists(filepath.Join(dataDir, "globalgamemanagers.assets")) {
			info.Engine = EngineUnity
			info.Confidence = 0.9
			info.DataPath = dataDir
			info.Notes = append(info.Notes, "Found globalgamemanagers in "+filepath.Base(dataDir))
		}
	} else if hasFileWithSuffix(root, ".unity3d") {
		info.Engine = EngineUnity
		info.Confidence = 0.85
		info.Notes = append(info.Notes, "Found .unity3d bundle files")
	}

	if info.Engine != EngineUnity {
		return false
	}

	// Determine backend: Mono vs IL2CPP
	dataDir := info.DataPath
	if dataDir == "" {
		dataDir = findDataDir(root)
	}

	if dataDir != "" {
		managedDir := filepath.Join(dataDir, "Managed")
		if dirExists(managedDir) {
			info.Backend = BackendMono
			info.Notes = append(info.Notes, "Backend: Mono (Managed/ folder found)")
		}
	}

	// IL2CPP: GameAssembly.dll in root or data dir
	if fileExists(filepath.Join(root, "GameAssembly.dll")) {
		info.Backend = BackendIL2CPP
		info.Notes = append(info.Notes, "Backend: IL2CPP (GameAssembly.dll found)")
	} else if dataDir != "" && fileExists(filepath.Join(dataDir, "GameAssembly.dll")) {
		info.Backend = BackendIL2CPP
		info.Notes = append(info.Notes, "Backend: IL2CPP")
	}

	// Try to extract version from globalgamemanagers
	if dataDir != "" {
		version := readUnityVersion(dataDir)
		if version != "" {
			info.Version = version
		}
	}

	return true
}

// readUnityVersion reads the Unity version string from globalgamemanagers.
// The version is stored as a length-prefixed string near the start of the file.
func readUnityVersion(dataDir string) string {
	ggm := filepath.Join(dataDir, "globalgamemanagers")
	if !fileExists(ggm) {
		ggm = filepath.Join(dataDir, "globalgamemanagers.assets")
	}
	if !fileExists(ggm) {
		return ""
	}

	data, err := os.ReadFile(ggm)
	if err != nil || len(data) < 30 {
		return ""
	}

	// The header format: [int version] [int revision] [string buildVersion]
	// Build version is a length-prefixed string at offset 8 (after two ints).
	// Actually the format varies. Try scanning for a version pattern.
	// Unity versions look like "2022.3.x", "2021.3.x", "5.6.x", etc.
	content := string(data)
	for i := 0; i < len(content)-6; i++ {
		// Look for patterns like "20XX.X" or "5.X" or "2019.X"
		if (content[i] >= '2' && content[i] <= '2') && content[i+1] == '0' {
			end := i + 2
			for end < len(content) && (content[end] == '.' || (content[end] >= '0' && content[end] <= '9')) {
				end++
			}
			ver := content[i:end]
			if len(ver) >= 5 && strings.Count(ver, ".") >= 1 {
				// Validate it looks like a version
				parts := strings.Split(ver, ".")
				if len(parts) >= 2 {
					return ver
				}
			}
		}
	}

	// Fallback: read from the binary format
	// Header: 4 bytes (header size), 4 bytes (version), 4 bytes (revision)
	// Then: 4 bytes (string length), N bytes (version string)
	if len(data) >= 20 {
		// Try different header sizes
		for _, hdrSize := range []int{0, 4, 8} {
			pos := hdrSize
			if pos+8 > len(data) {
				continue
			}
			// Skip version (4) + revision (4)
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

func detectUnreal(info *GameInfo) bool {
	root := info.GameRoot

	// Check for .pak files
	pakFiles := walkDir(root, 3, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".pak")
	})

	// Check for typical UE directory structure
	hasContentDir := findSubdir(root, "Content") != "" ||
		walkDir(root, 3, func(p string) bool {
			return filepath.Base(p) == "Content"
		}) != nil

	// Check for -Win64-Shipping.exe pattern
	hasShippingExe := walkDir(root, 3, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), "-win64-shipping.exe")
	}) != nil

	// Check for .uasset / .umap files
	hasUasset := walkDir(root, 3, func(p string) bool {
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".uasset" || ext == ".umap" || ext == ".uexp"
	}) != nil

	// Check for Engine folder
	hasEngineDir := findSubdir(root, "Engine") != ""

	score := 0
	if len(pakFiles) > 0 {
		score += 3
		info.Notes = append(info.Notes, fmt.Sprintf("Found %d .pak files", len(pakFiles)))
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
		info.Engine = EngineUnreal
		info.Confidence = float64(score) / 7.0
		if info.Confidence > 1.0 {
			info.Confidence = 1.0
		}

		// Try to detect UE4 vs UE5
		version := detectUnrealVersion(root, pakFiles)
		info.Version = version

		// Find .locres files for localization
		locresFiles := walkDir(root, 5, func(p string) bool {
			return strings.HasSuffix(strings.ToLower(p), ".locres")
		})
		if len(locresFiles) > 0 {
			info.Notes = append(info.Notes, fmt.Sprintf("Found %d .locres files", len(locresFiles)))
			info.DataPath = filepath.Dir(locresFiles[0])
		}

		return true
	}

	return false
}

func detectUnrealVersion(root string, pakFiles []string) string {
	// UE5: game folder structure has "Phoenix" or version markers
	// UE4: typically 4.xx
	// Try to find version from directory names or config files
	ue5Markers := []string{"Nanite", "Lumen", "VirtualShadowMaps"}
	ue4Markers := []string{"4.27", "4.26", "4.25", "4.24", "4.23", "4.22", "4.21", "4.20"}

	// Check for UE5-specific features
	for _, marker := range ue5Markers {
		if walkDir(root, 4, func(p string) bool {
			return strings.Contains(strings.ToLower(p), strings.ToLower(marker))
		}) != nil {
			return "UE5"
		}
	}

	// Check config files for version
	configDir := findSubdir(root, "Config")
	if configDir != "" {
		defaultEngine := filepath.Join(configDir, "DefaultEngine.ini")
		if fileExists(defaultEngine) {
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
		return "UE4/UE5 (unspecified)"
	}

	return ""
}

// ── Godot ───────────────────────────────────────────────────────────

func detectGodot(info *GameInfo) bool {
	root := info.GameRoot

	// Check for .pck file
	pckFiles := walkDir(root, 2, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".pck")
	})

	// Check for .godot project file (Godot 4.x)
	hasGodotFile := walkDir(root, 1, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".godot")
	}) != nil

	// Check for project.godot (Godot 3.x/4.x)
	hasProjectGodot := fileExists(filepath.Join(root, "project.godot"))

	if len(pckFiles) > 0 || hasGodotFile || hasProjectGodot {
		info.Engine = EngineGodot
		info.Confidence = 0.9
		info.Notes = append(info.Notes, fmt.Sprintf("Found %d .pck files", len(pckFiles)))

		// Determine Godot version
		if hasProjectGodot {
			data, err := os.ReadFile(filepath.Join(root, "project.godot"))
			if err == nil {
				content := string(data)
				if strings.Contains(content, "config_version=5") {
					info.Version = "4.x"
				} else if strings.Contains(content, "config_version=4") {
					info.Version = "3.x"
				}
			}
		}

		if info.Version == "" && len(pckFiles) > 0 {
			// Read version from PCK header
			info.Version = readPCKVersion(pckFiles[0])
		}

		return true
	}

	return false
}

func readPCKVersion(pckPath string) string {
	data, err := os.ReadFile(pckPath)
	if err != nil || len(data) < 8 {
		return ""
	}
	// PCK magic: "GDPC"
	if string(data[:4]) == "GDPC" {
		// Pack version at offset 4
		packVersion := binary.LittleEndian.Uint32(data[4:8])
		switch packVersion {
		case 1:
			return "3.x"
		case 2:
			return "4.x"
		default:
			return fmt.Sprintf("pack_ver=%d", packVersion)
		}
	}
	return ""
}

// ── RPG Maker ────────────────────────────────────────────────────────

func detectRPGMaker(info *GameInfo) bool {
	root := info.GameRoot

	// RPG Maker MV/MZ: www/ folder with package.json and data/*.json
	wwwDir := findSubdir(root, "www")
	if wwwDir != "" {
		if fileExists(filepath.Join(wwwDir, "package.json")) ||
			dirExists(filepath.Join(wwwDir, "data")) {
			info.Engine = EngineRPGMaker
			info.DataPath = wwwDir
			info.Confidence = 0.95
			info.Notes = append(info.Notes, "Found www/ directory (RPG Maker MV/MZ)")

			// Distinguish MV vs MZ
			if fileExists(filepath.Join(wwwDir, "package.json")) {
				data, err := os.ReadFile(filepath.Join(wwwDir, "package.json"))
				if err == nil {
					content := string(data)
					if strings.Contains(content, "mz") || strings.Contains(content, "MZ") {
						info.Version = "MZ"
					} else {
						info.Version = "MV"
					}
				}
			}
			if info.Version == "" {
				// MZ uses .js files in js/ folder with different structure
				if dirExists(filepath.Join(wwwDir, "js", "rmmz_core.js")) {
					info.Version = "MZ"
				} else {
					info.Version = "MV"
				}
			}

			return true
		}
	}

	// RPG Maker VX Ace / VX / XP: *.rvdata2 / *.rvdata / *.rxdata
	rpgFiles := walkDir(root, 2, func(p string) bool {
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".rvdata2" || ext == ".rvdata" || ext == ".rxdata"
	})
	if len(rpgFiles) > 0 {
		info.Engine = EngineRPGMaker
		info.Confidence = 0.9
		ext := strings.ToLower(filepath.Ext(rpgFiles[0]))
		switch ext {
		case ".rvdata2":
			info.Version = "VX Ace"
		case ".rvdata":
			info.Version = "VX"
		case ".rxdata":
			info.Version = "XP"
		}
		info.Notes = append(info.Notes, fmt.Sprintf("Found %d %s files", len(rpgFiles), ext))
		info.DataPath = filepath.Dir(rpgFiles[0])
		return true
	}

	return false
}

// ── GameMaker ────────────────────────────────────────────────────────

func detectGameMaker(info *GameInfo) bool {
	root := info.GameRoot

	// GameMaker: data.win file
	if fileExists(filepath.Join(root, "data.win")) {
		info.Engine = EngineGameMaker
		info.Confidence = 0.9
		info.Notes = append(info.Notes, "Found data.win (GameMaker)")
		info.DataPath = root

		// Try to detect GMS version
		data, err := os.ReadFile(filepath.Join(root, "data.win"))
		if err == nil && len(data) > 0 {
			content := string(data)
			if strings.Contains(content, "GameMaker Studio 2") {
				info.Version = "GMS2"
			} else if strings.Contains(content, "GameMaker Studio") {
				info.Version = "GMS1"
			} else {
				info.Version = "GM"
			}
		}
		return true
	}

	return false
}

// ── Ren'Py ───────────────────────────────────────────────────────────

func detectRenPy(info *GameInfo) bool {
	root := info.GameRoot

	// Ren'Py: game/ folder with .rpa/.rpy/.rpyc files
	gameDir := findSubdir(root, "game")
	if gameDir != "" {
		rpaFiles := walkDir(gameDir, 1, func(p string) bool {
			ext := strings.ToLower(filepath.Ext(p))
			return ext == ".rpa" || ext == ".rpy" || ext == ".rpyc"
		})
		if len(rpaFiles) > 0 {
			info.Engine = EngineRenPy
			info.Confidence = 0.95
			info.DataPath = gameDir
			info.Notes = append(info.Notes, fmt.Sprintf("Found %d Ren'Py script files", len(rpaFiles)))
			return true
		}
	}

	// Check for lib/ folder (Ren'Py runtime)
	libDir := findSubdir(root, "lib")
	if libDir != "" && findSubdir(root, "game") != "" {
		info.Engine = EngineRenPy
		info.Confidence = 0.85
		info.DataPath = findSubdir(root, "game")
		info.Notes = append(info.Notes, "Found lib/ + game/ directories (Ren'Py)")
		return true
	}

	return false
}


// ── Adobe AIR / Flash ────────────────────────────────────────────────

func detectAdobeAIR(info *GameInfo) bool {
	root := info.GameRoot

	// Adobe AIR: .swf files, Adobe AIR folder, .swz files, mimetype file
	hasAIRDir := findSubdir(root, "Adobe AIR") != ""
	hasSWF := hasFileWithSuffix(root, ".swf")
	hasSWZ := hasFileWithSuffix(root, ".swz")

	if hasAIRDir && (hasSWF || hasSWZ) {
		info.Engine = EngineAdobeAIR
		info.Confidence = 0.9
		info.DataPath = root
		info.Notes = append(info.Notes, "Found Adobe AIR runtime + SWF/SWZ files")
		return true
	}

	return false
}

// ── Source Engine ────────────────────────────────────────────────────

func detectSource(info *GameInfo) bool {
	root := info.GameRoot

	// Source: .vpk files, .bsp files, bin/ with source DLLs
	vpkFiles := walkDir(root, 2, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".vpk")
	})
	bspFiles := walkDir(root, 3, func(p string) bool {
		return strings.HasSuffix(strings.ToLower(p), ".bsp")
	})

	if len(vpkFiles) > 0 || len(bspFiles) > 0 {
		info.Engine = EngineSource
		info.Confidence = 0.85
		info.Notes = append(info.Notes, fmt.Sprintf("Found %d .vpk, %d .bsp files", len(vpkFiles), len(bspFiles)))
		info.DataPath = root
		return true
	}

	return false
}

// ── GoldSrc (Half-Life 1 era) ────────────────────────────────────────

func detectGoldSrc(info *GameInfo) bool {
	root := info.GameRoot

	// GoldSrc: liblist.gam file (may be in root or a mod subdirectory like cstrike/)
	// Also check for hl.exe (Half-Life engine executable)
	liblistFiles := walkDir(root, 2, func(p string) bool {
		return strings.EqualFold(filepath.Base(p), "liblist.gam")
	})
	if len(liblistFiles) > 0 {
		info.Engine = EngineGoldSrc
		info.Confidence = 0.9
		info.DataPath = filepath.Dir(liblistFiles[0])
		info.Notes = append(info.Notes, "Found liblist.gam (GoldSrc)")
		return true
	}

	// Check for hl.exe (Half-Life/GoldSrc engine)
	hlExe := walkDir(root, 1, func(p string) bool {
		name := strings.ToLower(filepath.Base(p))
		return name == "hl.exe" || name == "hlds.exe"
	})
	if len(hlExe) > 0 {
		// Also check for .wad files
		wadFiles := walkDir(root, 3, func(p string) bool {
			return strings.HasSuffix(strings.ToLower(p), ".wad")
		})
		if len(wadFiles) > 0 {
			info.Engine = EngineGoldSrc
			info.Confidence = 0.85
			info.DataPath = root
			info.Notes = append(info.Notes, fmt.Sprintf("Found hl.exe + %d .wad files (GoldSrc)", len(wadFiles)))
			return true
		}
	}

	return false
}
