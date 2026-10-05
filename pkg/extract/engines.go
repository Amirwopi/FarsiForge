package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/project"
	"farsiforge/pkg/tools"
)

// UnityExtractor extracts strings from Unity games using UnityPy.
type UnityExtractor struct{}

func (e *UnityExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		DialogueExtraction: true,
		FontExtraction:     true,
		NeedsExternalTool:  true,
		ToolName:           "py_UnityPy",
	}
}

func (e *UnityExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	if reg.Python == "" {
		return fmt.Errorf("Python not found")
	}
	if !reg.IsAvailable("py_UnityPy") {
		return fmt.Errorf("UnityPy not installed. Run: pip install UnityPy")
	}

	// Determine the data directory
	dataDir := info.DataPath
	if dataDir == "" {
		dataDir = findDataDir(info.GameRoot)
	}
	if dataDir == "" {
		dataDir = info.GameRoot
	}

	// Run the UnityPy extraction script
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return err
	}

	outputFile := filepath.Join(workDir, "unity_strings.json")
	script := buildUnityPyScript(dataDir, outputFile)

	cmd := exec.Command(reg.Python, "-c", script)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("UnityPy extraction failed: %w\nOutput: %s", err, string(output))
	}

	// Parse the results
	data, err := os.ReadFile(outputFile)
	if err != nil {
		return fmt.Errorf("read extraction output: %w", err)
	}

	var results []unityString
	if err := json.Unmarshal(data, &results); err != nil {
		return fmt.Errorf("parse extraction output: %w", err)
	}

	// Add entries to project
	for _, s := range results {
		relFile, _ := filepath.Rel(info.GameRoot, s.File)
		proj.AddEntry(project.StringEntry{
			Source:  s.Text,
			File:    relFile,
			Path:    s.Path,
			Context: s.Context,
			Status:  project.StatusUntranslated,
		})
	}

	return nil
}

type unityString struct {
	Text    string `json:"text"`
	File    string `json:"file"`
	Path    string `json:"path"`
	Context string `json:"context"`
}

// buildUnityPyScript creates a Python script that uses UnityPy to extract
// all text strings from Unity asset files.
func buildUnityPyScript(dataDir, outputFile string) string {
	return fmt.Sprintf(`
import UnityPy
import sys
sys.stdout.reconfigure(encoding='utf-8')
import json
import os
import sys

results = []
data_dir = %q
output_file = %q

# Collect all asset files
asset_files = []
for root, dirs, files in os.walk(data_dir):
    for f in files:
        ext = f.lower().split('.')[-1] if '.' in f else ''
        if ext in ('assets', 'bundle', 'unity3d', 'resS', 'resource'):
            asset_files.append(os.path.join(root, f))
    # Don't go too deep
    rel = os.path.relpath(root, data_dir)
    if rel.count(os.sep) > 4:
        dirs[:] = []

# Also check for .assets files in root
for f in os.listdir(data_dir):
    if f.lower().endswith('.assets'):
        asset_files.append(os.path.join(data_dir, f))

seen_texts = set()
for af in asset_files:
    try:
        env = UnityPy.load(af)
        for obj in env.objects:
            try:
                if obj.type.name == "TextAsset":
                    data = obj.read()
                    text = ""
                    # Try different field names for different UnityPy versions
                    if hasattr(data, 'm_Script'):
                        text = data.m_Script
                    elif hasattr(data, 'text'):
                        text = data.text
                    elif hasattr(data, 'm_Text'):
                        text = data.m_Text
                    if isinstance(text, bytes):
                        text = text.decode('utf-8', errors='replace')
                    if text and len(text.strip()) > 0:
                        # Split into lines and add each meaningful line
                        for line in text.strip().split('\\n'):
                            line = line.strip()
                            if len(line) > 1 and line not in seen_texts:
                                seen_texts.add(line)
                                results.append({
                                    "text": line,
                                    "file": af,
                                    "path": f"TextAsset/{{data.m_Name if hasattr(data, 'm_Name') else '?'}}",
                                    "context": "text_asset"
                                })
                elif obj.type.name == "MonoBehaviour":
                    # Try to extract strings from MonoBehaviour via typetree
                    try:
                        tree = obj.read_typetree()
                        extract_strings_from_tree(tree, af, obj.path_id, results, seen_texts)
                    except:
                        pass
                elif obj.type.name == "Font":
                    data = obj.read()
                    name = getattr(data, 'm_Name', '') or ''
                    results.append({
                        "text": f"[FONT] {name}",
                        "file": af,
                        "path": f"Font/{name}",
                        "context": "font_info"
                    })
            except Exception as e:
                continue
    except Exception as e:
        continue

def extract_strings_from_tree(tree, file_path, path_id, results, seen):
    """Recursively extract string values from a typetree dict."""
    if isinstance(tree, dict):
        for key, value in tree.items():
            if isinstance(value, str) and len(value) > 1 and not key.startswith('_'):
                if value not in seen and not value.startswith('UnityEngine'):
                    seen.add(value)
                    results.append({
                        "text": value,
                        "file": file_path,
                        "path": f"MonoBehaviour/{path_id}/{key}",
                        "context": "mono_behaviour"
                    })
            elif isinstance(value, (dict, list)):
                extract_strings_from_tree(value, file_path, path_id, results, seen)
    elif isinstance(tree, list):
        for item in tree:
            if isinstance(item, (dict, list)):
                extract_strings_from_tree(item, file_path, path_id, results, seen)

with open(output_file, 'w', encoding='utf-8') as f:
    json.dump(results, f, ensure_ascii=False, indent=2)

print(f"Extracted {len(results)} strings from {len(asset_files)} files")
`, dataDir, outputFile)
}

func findDataDir(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), "_data") {
			return filepath.Join(root, e.Name())
		}
	}
	return ""
}

// ── Unreal Extractor ──────────────────────────────────────────────────

type UnrealExtractor struct{}

func (e *UnrealExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		NeedsExternalTool:  true,
		ToolName:           "unreal_locres",
	}
}

func (e *UnrealExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	if reg.UnrealLocres == "" {
		return fmt.Errorf("UnrealLocres.exe not found in Tools directory")
	}

	// Find all .locres files
	var locresFiles []string
	filepath.Walk(info.GameRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".locres") {
			locresFiles = append(locresFiles, path)
		}
		return nil
	})

	if len(locresFiles) == 0 {
		return fmt.Errorf("no .locres files found. The game may need .pak extraction first (use repak)")
	}

	// Export each .locres to JSON using UnrealLocres
	workDir, _ := proj.EnsureWorkingDir()
	for _, locres := range locresFiles {
		outFile := filepath.Join(workDir, filepath.Base(locres)+".json")
		_, err := tools.Run(reg.UnrealLocres, "export", locres, "-json", outFile)
		if err != nil {
			continue
		}
		// Parse and add entries
		data, err := os.ReadFile(outFile)
		if err != nil {
			continue
		}
		// UnrealLocres exports JSON with string entries
		var entries map[string]interface{}
		if err := json.Unmarshal(data, &entries); err != nil {
			continue
		}
		relFile, _ := filepath.Rel(info.GameRoot, locres)
		for key, val := range entries {
			text, ok := val.(string)
			if !ok {
				continue
			}
			proj.AddEntry(project.StringEntry{
				Source:  text,
				File:    relFile,
				Path:    key,
				Context: "locres",
				Status:  project.StatusUntranslated,
			})
		}
	}

	return nil
}

// ── Godot Extractor ───────────────────────────────────────────────────

type GodotExtractor struct{}

func (e *GodotExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		NeedsExternalTool:  true,
		ToolName:           "gdre_tools",
	}
}

func (e *GodotExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	if reg.GDRETools == "" {
		return fmt.Errorf("gdre_tools.exe not found in Tools directory")
	}

	workDir, _ := proj.EnsureWorkingDir()

	// Find .pck files
	var pckFiles []string
	filepath.Walk(info.GameRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".pck") {
			pckFiles = append(pckFiles, path)
		}
		return nil
	})

	if len(pckFiles) == 0 {
		return fmt.Errorf("no .pck files found")
	}

	// Extract PCK using gdre_tools
	for _, pck := range pckFiles {
		outDir := filepath.Join(workDir, filepath.Base(pck)+"_extracted")
		_, err := tools.Run(reg.GDRETools, "--recover", pck, "--output-dir", outDir)
		if err != nil {
			continue
		}

		// Scan extracted files for text content
		filepath.Walk(outDir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".csv" || ext == ".json" || ext == ".txt" || ext == ".po" || ext == ".tres" {
				extractTextFromFile(path, proj, info.GameRoot)
			}
			return nil
		})
	}

	return nil
}

// ── GameMaker Extractor ───────────────────────────────────────────────

type GameMakerExtractor struct{}

func (e *GameMakerExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		NeedsExternalTool:  true,
		ToolName:           "undertale_mod_tool",
	}
}

func (e *GameMakerExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	return fmt.Errorf("GameMaker extraction requires UndertaleModTool (not yet automated)")
}

// ── Ren'Py Extractor ──────────────────────────────────────────────────

type RenPyExtractor struct{}

func (e *RenPyExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction:     true,
		DialogueExtraction: true,
	}
}

func (e *RenPyExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	// Scan .rpy files for dialogue strings
	gameDir := info.DataPath
	if gameDir == "" {
		gameDir = info.GameRoot
	}

	filepath.Walk(gameDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".rpy" {
			extractRenPyDialogue(path, proj, info.GameRoot)
		}
		return nil
	})

	return nil
}

// ── Source Extractor ──────────────────────────────────────────────────

type SourceExtractor struct{}

func (e *SourceExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction: true,
	}
}

func (e *SourceExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	// Source engine uses .txt files for localization (closecaption_english.txt, etc.)
	// Also .res files for UI
	filepath.Walk(info.GameRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		name := strings.ToLower(fi.Name())
		if strings.Contains(name, "english") && (strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".res")) {
			extractTextFromFile(path, proj, info.GameRoot)
		}
		return nil
	})
	return nil
}

// ── Adobe AIR Extractor ───────────────────────────────────────────────

type AdobeAIRExtractor struct{}

func (e *AdobeAIRExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction: true,
	}
}

func (e *AdobeAIRExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	// Adobe AIR: check languages/ folder for XML/JSON localization files
	langDir := filepath.Join(info.GameRoot, "languages")
	if dirExists(langDir) {
		filepath.Walk(langDir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".xml" || ext == ".json" || ext == ".txt" {
				extractTextFromFile(path, proj, info.GameRoot)
			}
			return nil
		})
	}

	// Also check for SWF text (requires SWF decompiler)
	return nil
}

// ── Text File Extractor (generic) ─────────────────────────────────────

type TextFileExtractor struct{}

func (e *TextFileExtractor) Capabilities() Capabilities {
	return Capabilities{
		TextExtraction: true,
	}
}

func (e *TextFileExtractor) Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	// Scan for common text file formats
	extensions := []string{".json", ".csv", ".txt", ".xml", ".ini", ".lang", ".po", ".properties"}
	extSet := make(map[string]bool)
	for _, e := range extensions {
		extSet[e] = true
	}

	filepath.Walk(info.GameRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if extSet[ext] {
			// Skip very large files
			if fi.Size() > 10*1024*1024 { // 10MB
				return nil
			}
			extractTextFromFile(path, proj, info.GameRoot)
		}
		return nil
	})

	return nil
}

// extractTextFromFile reads a text file and adds its content as string entries.
func extractTextFromFile(path string, proj *project.Project, gameRoot string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	relFile, _ := filepath.Rel(gameRoot, path)
	content := string(data)
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		extractFromJSON(content, relFile, proj)
	case ".csv":
		extractFromCSV(content, relFile, proj)
	case ".po":
		extractFromPO(content, relFile, proj)
	default:
		// For plain text, add each non-empty line
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 2 && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
				proj.AddEntry(project.StringEntry{
					Source:  line,
					File:    relFile,
					Path:    fmt.Sprintf("line:%d", i+1),
					Context: "text",
					Status:  project.StatusUntranslated,
				})
			}
		}
	}
}

func extractFromJSON(content, file string, proj *project.Project) {
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return
	}
	walkJSON(data, "", file, proj, make(map[string]bool))
}

func walkJSON(data interface{}, prefix, file string, proj *project.Project, seen map[string]bool) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			walkJSON(val, newPrefix, file, proj, seen)
		}
	case []interface{}:
		for i, item := range v {
			walkJSON(item, fmt.Sprintf("%s[%d]", prefix, i), file, proj, seen)
		}
	case string:
		if len(v) > 1 && !seen[v] && !isLikelyKey(v) {
			seen[v] = true
			proj.AddEntry(project.StringEntry{
				Source:  v,
				File:    file,
				Path:    prefix,
				Context: "json",
				Status:  project.StatusUntranslated,
			})
		}
	}
}

func isLikelyKey(s string) bool {
	// Heuristic: if it's all lowercase, short, and has no spaces, it's likely a key not a value
	if len(s) < 3 || len(s) > 200 {
		return false
	}
	if !strings.Contains(s, " ") && !strings.Contains(s, ".") && strings.ToLower(s) == s {
		// Could be a key like "game_title" — but also could be a short word
		if strings.Contains(s, "_") || strings.Contains(s, "-") {
			return true
		}
	}
	return false
}

func extractFromCSV(content, file string, proj *project.Project) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 { // skip header
			continue
		}
		// Simple CSV parse
		fields := strings.Split(line, ",")
		if len(fields) >= 2 {
			proj.AddEntry(project.StringEntry{
				Source:  fields[1],
				File:    file,
				Path:    fmt.Sprintf("row:%d,col:1", i+1),
				Context: "csv",
				Status:  project.StatusUntranslated,
			})
		}
	}
}

func extractFromPO(content, file string, proj *project.Project) {
	// .po format: msgid "source" / msgstr "translation"
	lines := strings.Split(content, "\n")
	var currentMsgID string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "msgid \"") {
			currentMsgID = strings.TrimSuffix(strings.TrimPrefix(line, "msgid \""), "\"")
		} else if strings.HasPrefix(line, "msgstr \"") && currentMsgID != "" {
			if currentMsgID != "" && len(currentMsgID) > 1 {
				proj.AddEntry(project.StringEntry{
					Source:  currentMsgID,
					File:    file,
					Path:    "po:msgid",
					Context: "po",
					Status:  project.StatusUntranslated,
				})
			}
			currentMsgID = ""
		}
	}
}

func extractRenPyDialogue(path string, proj *project.Project, gameRoot string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	relFile, _ := filepath.Rel(gameRoot, path)
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Ren'Py dialogue patterns:
		// "text" 
		// character "text"
		// 'text'
		if (strings.HasPrefix(line, "\"") || strings.HasPrefix(line, "'")) && len(line) > 2 {
			text := strings.Trim(line, "\"'")
			if len(text) > 1 {
				proj.AddEntry(project.StringEntry{
					Source:  text,
					File:    relFile,
					Path:    fmt.Sprintf("line:%d", i+1),
					Context: "dialogue",
					Status:  project.StatusUntranslated,
				})
			}
		}
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
