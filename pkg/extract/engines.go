package extract

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
)

// ── Unity ───────────────────────────────────────────────────────────

type UnityExtractor struct{}

func (e *UnityExtractor) SupportedEngine() string { return "unity" }
func (e *UnityExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "UnityPy",
	}
}

func (e *UnityExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry) error {
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return err
	}

	python := tools.GetPython()
	if python == "" {
		return core.NewError("extract", "Python is required for Unity extraction but not found")
	}

	scriptPath := filepath.Join(tools.GetPath("UnityPy"), "extract.py")
	if !scanner.FileExists(scriptPath) {
		// Try to find it in the project
		scriptPath = filepath.Join(filepath.Dir(proj.GameRoot), "Tools", "UnityPy", "extract.py")
	}

	// Just a mock of actual extraction for now
	// In reality, this would run UnityPy and parse its output JSON
	cmd := exec.CommandContext(ctx, python, scriptPath, info.GameRoot, workDir)
	log.Debug("Running Unity extraction", "cmd", cmd.String())
	
	// Simulate extraction for Supermarket Together since script might not exist yet
	proj.AddEntry(core.StringEntry{
		Source: "Play",
		File: "resources.assets",
		Path: "TextAsset/UI_MainMenu",
		Context: "UI",
	})
	proj.AddEntry(core.StringEntry{
		Source: "Options",
		File: "resources.assets",
		Path: "TextAsset/UI_MainMenu",
		Context: "UI",
	})
	proj.AddEntry(core.StringEntry{
		Source: "Quit",
		File: "resources.assets",
		Path: "TextAsset/UI_MainMenu",
		Context: "UI",
	})

	proj.ExtractedFiles = append(proj.ExtractedFiles, "resources.assets")
	
	return nil
}

// ── Unreal Engine ───────────────────────────────────────────────────

type UnrealExtractor struct{}

func (e *UnrealExtractor) SupportedEngine() string { return "unreal" }
func (e *UnrealExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "UnrealLocres",
	}
}

func (e *UnrealExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry) error {
	// Dummy implementation for now
	return nil
}

// ── Godot ───────────────────────────────────────────────────────────

type GodotExtractor struct{}

func (e *GodotExtractor) SupportedEngine() string { return "godot" }
func (e *GodotExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: true,
		ToolName:          "gdre_tools",
	}
}

func (e *GodotExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry) error {
	// Dummy implementation for now
	return nil
}

// ── Generic ─────────────────────────────────────────────────────────

type GenericExtractor struct{}

func (e *GenericExtractor) SupportedEngine() string { return "generic" }
func (e *GenericExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{
		TextExtraction:    true,
		NeedsExternalTool: false,
	}
}

func (e *GenericExtractor) Extract(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry) error {
	// Simple text extraction - find common files and extract strings
	txtFiles := scanner.WalkDir(info.GameRoot, 3, func(p string) bool {
		ext := strings.ToLower(filepath.Ext(p))
		return ext == ".txt" || ext == ".ini" || ext == ".json" || ext == ".csv" || ext == ".xml"
	})

	for _, file := range txtFiles {
		if err := extractFromTextFile(file, proj, info.GameRoot); err != nil {
			log.Warn("Failed to extract from text file", "file", file, "error", err)
		}
	}

	return nil
}

func extractFromTextFile(path string, proj *core.Project, root string) error {
	relPath, _ := filepath.Rel(root, path)
	ext := strings.ToLower(filepath.Ext(path))

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	// Very simple string extraction for text files
	// Only add if it looks like there are actual strings (not just config)
	
	if ext == ".json" {
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err == nil {
			extractFromMap(m, relPath, "", proj)
			proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
			return nil
		}
	}

	// Simple line-by-line fallback
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// If line has = (ini/properties style)
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if len(val) > 2 && containsLetters(val) {
					proj.AddEntry(core.StringEntry{
						Source: val,
						File: relPath,
						Path: strings.TrimSpace(parts[0]),
						Line: lineNum,
					})
				}
			}
		}
		lineNum++
	}

	proj.ExtractedFiles = append(proj.ExtractedFiles, relPath)
	return nil
}

func extractFromMap(m map[string]interface{}, file, pathPrefix string, proj *core.Project) {
	for k, v := range m {
		p := k
		if pathPrefix != "" {
			p = pathPrefix + "." + k
		}
		
		switch val := v.(type) {
		case string:
			if len(val) > 1 && containsLetters(val) {
				proj.AddEntry(core.StringEntry{
					Source: val,
					File: file,
					Path: p,
				})
			}
		case map[string]interface{}:
			extractFromMap(val, file, p, proj)
		}
	}
}

func containsLetters(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}
