package inject

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/project"
	"farsiforge/pkg/tools"
)

// ── Unity Injector ───────────────────────────────────────────────────

type UnityInjector struct{}

func (i *UnityInjector) Capabilities() Capabilities {
	return Capabilities{
		TextInjection:      true,
		FontInjection:      true,
		NeedsExternalTool:  true,
		ToolName:           "py_UnityPy",
	}
}

func (i *UnityInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	if reg.Python == "" || !reg.IsAvailable("py_UnityPy") {
		return nil, fmt.Errorf("Python + UnityPy required for Unity injection")
	}

	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return nil, err
	}

	// Build translation map
	transMap := BuildTranslationMap(proj, opts)

	// Write translation map to JSON
	mapFile := filepath.Join(workDir, "translation_map.json")
	mapData, _ := json.MarshalIndent(transMap, "", "  ")
	os.WriteFile(mapFile, mapData, 0644)

	// Determine data directory
	dataDir := info.DataPath
	if dataDir == "" {
		dataDir = findDataDir(info.GameRoot)
	}
	if dataDir == "" {
		dataDir = info.GameRoot
	}

	// Create backup directory
	backupDir := filepath.Join(workDir, "backup")
	os.MkdirAll(backupDir, 0755)

	// Run UnityPy injection script
	script := buildUnityInjectScript(dataDir, mapFile, backupDir, workDir)
	cmd := exec.Command(reg.Python, "-c", script)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Unity injection failed: %w\nOutput: %s", err, string(output))
	}

	// Parse modified files list
	modifiedFile := filepath.Join(workDir, "modified_files.json")
	modData, err := os.ReadFile(modifiedFile)
	if err != nil {
		return nil, fmt.Errorf("read modified files list: %w", err)
	}

	var modified []string
	json.Unmarshal(modData, &modified)

	return modified, nil
}

func buildUnityInjectScript(dataDir, mapFile, backupDir, workDir string) string {
	return fmt.Sprintf(`
import UnityPy
import json
import os
import shutil

# Load translation map
with open(%q, 'r', encoding='utf-8') as f:
    trans_map = json.load(f)

modified_files = []

# Collect asset files
asset_files = []
for root, dirs, files in os.walk(%q):
    for fn in files:
        ext = fn.lower().rsplit('.', 1)[-1] if '.' in fn else ''
        if ext in ('assets', 'bundle', 'unity3d'):
            asset_files.append(os.path.join(root, fn))
    rel = os.path.relpath(root, %q)
    if rel.count(os.sep) > 4:
        dirs[:] = []

for af in asset_files:
    try:
        env = UnityPy.load(af)
        modified = False
        
        for obj in env.objects:
            try:
                if obj.type.name == "TextAsset":
                    data = obj.read()
                    text = ""
                    if hasattr(data, 'm_Script'):
                        text = data.m_Script
                    elif hasattr(data, 'text'):
                        text = data.text
                    elif hasattr(data, 'm_Text'):
                        text = data.m_Text
                    
                    if isinstance(text, bytes):
                        text = text.decode('utf-8', errors='replace')
                    
                    if text:
                        new_text = text
                        for src, trn in trans_map.items():
                            if src in new_text:
                                new_text = new_text.replace(src, trn)
                        
                        if new_text != text:
                            if hasattr(data, 'm_Script'):
                                data.m_Script = new_text.encode('utf-8')
                            elif hasattr(data, 'text'):
                                data.text = new_text
                            elif hasattr(data, 'm_Text'):
                                data.m_Text = new_text
                            data.save()
                            modified = True
                
                elif obj.type.name == "MonoBehaviour":
                    try:
                        tree = obj.read_typetree()
                        changed = replace_in_tree(tree, trans_map)
                        if changed:
                            obj.save_typetree(tree)
                            modified = True
                    except:
                        pass
            except Exception as e:
                continue
        
        if modified:
            # Backup original
            rel_path = os.path.relpath(af, %q)
            backup_path = os.path.join(%q, rel_path)
            os.makedirs(os.path.dirname(backup_path), exist_ok=True)
            if not os.path.exists(backup_path):
                shutil.copy2(af, backup_path)
            
            # Save modified file
            with open(af, 'wb') as f:
                f.write(env.file.save())
            modified_files.append(af)
    except Exception as e:
        continue

def replace_in_tree(tree, trans_map):
    changed = False
    if isinstance(tree, dict):
        for key, val in tree.items():
            if isinstance(val, str) and val in trans_map:
                tree[key] = trans_map[val]
                changed = True
            elif isinstance(val, (dict, list)):
                if replace_in_tree(val, trans_map):
                    changed = True
    elif isinstance(tree, list):
        for i, item in enumerate(tree):
            if isinstance(item, str) and item in trans_map:
                tree[i] = trans_map[item]
                changed = True
            elif isinstance(item, (dict, list)):
                if replace_in_tree(item, trans_map):
                    changed = True
    return changed

with open(%q, 'w', encoding='utf-8') as f:
    json.dump(modified_files, f, ensure_ascii=False, indent=2)

print(f"Modified {len(modified_files)} files")
`, mapFile, dataDir, dataDir, dataDir, backupDir, filepath.Join(workDir, "modified_files.json"))
}

// ── RPG Maker Injector ───────────────────────────────────────────────

type RPGMakerInjector struct{}

func (i *RPGMakerInjector) Capabilities() Capabilities {
	return Capabilities{
		TextInjection: true,
	}
}

func (i *RPGMakerInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	dataDir := info.DataPath
	if dataDir == "" {
		dataDir = filepath.Join(info.GameRoot, "www", "data")
		if !dirExists(dataDir) {
			dataDir = filepath.Join(info.GameRoot, "data")
		}
	}
	if !dirExists(dataDir) {
		return nil, fmt.Errorf("RPG Maker data directory not found")
	}

	// Build translation map: path → processed translation
	transMap := BuildTranslationMap(proj, opts)
	if len(transMap) == 0 {
		return nil, fmt.Errorf("no translations to inject")
	}

	// Also build path-based map for precision
	pathMap := make(map[string]string)
	for _, e := range proj.Entries {
		if (e.Status == project.StatusTranslated || e.Status == project.StatusApproved) && e.Translation != "" {
			processed := persian.Process(e.Translation, opts)
			pathMap[e.Path] = processed
		}
	}

	var modified []string

	// Process all JSON files in data directory
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}

	workDir, _ := proj.EnsureWorkingDir()
	backupDir := filepath.Join(workDir, "backup", "data")
	os.MkdirAll(backupDir, 0755)

	for _, e := range entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dataDir, e.Name())

		// Backup
		backupPath := filepath.Join(backupDir, e.Name())
		if !fileExists(backupPath) {
			copyFile(path, backupPath)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Replace strings in JSON content
		content := string(data)
		modified_content := content
		for src, trn := range transMap {
			// Escape for JSON string
			escSrc := jsonEscape(src)
			escTrn := jsonEscape(trn)
			modified_content = strings.ReplaceAll(modified_content, escSrc, escTrn)
		}

		if modified_content != content {
			os.WriteFile(path, []byte(modified_content), 0644)
			modified = append(modified, path)
		}
	}

	return modified, nil
}

// jsonEscape escapes a string for safe embedding in JSON content.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}

// ── Unreal Injector ──────────────────────────────────────────────────

type UnrealInjector struct{}

func (i *UnrealInjector) Capabilities() Capabilities {
	return Capabilities{
		TextInjection:      true,
		NeedsExternalTool:  true,
		ToolName:           "unreal_locres",
	}
}

func (i *UnrealInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	if reg.UnrealLocres == "" {
		return nil, fmt.Errorf("UnrealLocres.exe not found")
	}

	transMap := BuildTranslationMap(proj, opts)
	workDir, _ := proj.EnsureWorkingDir()

	var modified []string

	// Find all .locres files
	filepath.Walk(info.GameRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".locres") {
			return nil
		}

		// Export to JSON
		jsonFile := filepath.Join(workDir, filepath.Base(path)+".json")
		tools.Run(reg.UnrealLocres, "export", path, "-json", jsonFile)

		// Modify JSON with translations
		data, err := os.ReadFile(jsonFile)
		if err != nil {
			return nil
		}
		var entries map[string]interface{}
		if err := json.Unmarshal(data, &entries); err != nil {
			return nil
		}

		changed := false
		for key, val := range entries {
			if text, ok := val.(string); ok {
				if trn, exists := transMap[text]; exists {
					entries[key] = trn
					changed = true
				}
			}
		}

		if changed {
			// Write modified JSON
			modData, _ := json.MarshalIndent(entries, "", "  ")
			os.WriteFile(jsonFile, modData, 0644)

			// Import back to locres
			tools.Run(reg.UnrealLocres, "import", jsonFile, path)
			modified = append(modified, path)
		}

		return nil
	})

	return modified, nil
}

// ── Godot Injector ───────────────────────────────────────────────────

type GodotInjector struct{}

func (i *GodotInjector) Capabilities() Capabilities {
	return Capabilities{
		TextInjection:      true,
		NeedsExternalTool:  true,
		ToolName:           "gdre_tools",
	}
}

func (i *GodotInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	return nil, fmt.Errorf("Godot injection: extract PCK with gdre_tools, modify files, repack (not yet automated)")
}

// ── Ren'Py Injector ──────────────────────────────────────────────────

type RenPyInjector struct{}

func (i *RenPyInjector) Capabilities() Capabilities {
	return Capabilities{TextInjection: true}
}

func (i *RenPyInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	transMap := BuildTranslationMap(proj, opts)
	gameDir := info.DataPath
	if gameDir == "" {
		gameDir = info.GameRoot
	}

	var modified []string
	filepath.Walk(gameDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".rpy") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		modified_content := content
		for src, trn := range transMap {
			modified_content = strings.ReplaceAll(modified_content, "\""+src+"\"", "\""+trn+"\"")
			modified_content = strings.ReplaceAll(modified_content, "'"+src+"'", "'"+trn+"'")
		}

		if modified_content != content {
			os.WriteFile(path, []byte(modified_content), 0644)
			modified = append(modified, path)
		}
		return nil
	})

	return modified, nil
}

// ── Source Injector ──────────────────────────────────────────────────

type SourceInjector struct{}

func (i *SourceInjector) Capabilities() Capabilities {
	return Capabilities{TextInjection: true}
}

func (i *SourceInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	return injectTextFiles(info, proj, reg, opts, []string{".txt", ".res"})
}

// ── Adobe AIR Injector ───────────────────────────────────────────────

type AdobeAIRInjector struct{}

func (i *AdobeAIRInjector) Capabilities() Capabilities {
	return Capabilities{TextInjection: true}
}

func (i *AdobeAIRInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	langDir := filepath.Join(info.GameRoot, "languages")
	if dirExists(langDir) {
		return injectTextFilesInDir(langDir, info, proj, reg, opts, []string{".xml", ".json", ".txt"})
	}
	return nil, fmt.Errorf("no languages directory found")
}

// ── Text File Injector (generic) ─────────────────────────────────────

type TextFileInjector struct{}

func (i *TextFileInjector) Capabilities() Capabilities {
	return Capabilities{TextInjection: true}
}

func (i *TextFileInjector) Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	return injectTextFiles(info, proj, reg, opts, []string{".json", ".csv", ".txt", ".xml", ".ini", ".lang", ".po"})
}

func injectTextFiles(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options, extensions []string) ([]string, error) {
	return injectTextFilesInDir(info.GameRoot, info, proj, reg, opts, extensions)
}

func injectTextFilesInDir(rootDir string, info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options, extensions []string) ([]string, error) {
	transMap := BuildTranslationMap(proj, opts)
	if len(transMap) == 0 {
		return nil, fmt.Errorf("no translations to inject")
	}

	extSet := make(map[string]bool)
	for _, e := range extensions {
		extSet[e] = true
	}

	var modified []string
	filepath.Walk(rootDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !extSet[ext] {
			return nil
		}
		if fi.Size() > 10*1024*1024 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		modified_content := content
		for src, trn := range transMap {
			modified_content = strings.ReplaceAll(modified_content, src, trn)
		}

		if modified_content != content {
			os.WriteFile(path, []byte(modified_content), 0644)
			modified = append(modified, path)
		}
		return nil
	})

	return modified, nil
}

// ── Utility functions ────────────────────────────────────────────────

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

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
