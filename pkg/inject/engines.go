package inject

import (
	"context"
	"os/exec"
	"path/filepath"

	"farsiforge/pkg/backup"
	"farsiforge/pkg/core"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/scanner"
)

// ── Unity ───────────────────────────────────────────────────────────

type UnityInjector struct{}

func (i *UnityInjector) SupportedEngine() string { return "unity" }
func (i *UnityInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "UnityPy",
	}
}

func (i *UnityInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	res := &core.InjectionResult{}
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}

	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))
	
	// Create translations dictionary for injection
	translatedEntries := proj.FindTranslated()
	if len(translatedEntries) == 0 {
		res.Warnings = append(res.Warnings, "No translated entries found")
		return res, nil
	}

	// Process Persian text
	processed := make(map[string]string)
	for _, e := range translatedEntries {
		tr := e.Translation
		if opts.Reshape || opts.BidiReorder || opts.FixYeh || opts.PersianDigits {
			tr = persian.Process(tr, persian.Options{
				Reshape:       opts.Reshape,
				BidiReorder:   opts.BidiReorder,
				FixYeh:        opts.FixYeh,
				PersianDigits: opts.PersianDigits,
				ConvertPunct:  opts.ConvertPunct,
				DropDiacritics: opts.DropDiacritics,
			})
		}
		processed[e.Source] = tr
	}

	res.StringCount = len(processed)

	python := tools.GetPython()
	scriptPath := filepath.Join(tools.GetPath("UnityPy"), "inject.py")
	if !scanner.FileExists(scriptPath) {
		scriptPath = filepath.Join(filepath.Dir(proj.GameRoot), "Tools", "UnityPy", "inject.py")
	}

	// Backup target files
	for _, file := range proj.ExtractedFiles {
		_, _, err := backupMgr.BackupFile(file)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
		} else {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}

	// Mock injection command
	cmd := exec.CommandContext(ctx, python, scriptPath, info.GameRoot, workDir)
	log.Debug("Running Unity injection", "cmd", cmd.String())
	
	return res, nil
}

// ── Unreal Engine ───────────────────────────────────────────────────

type UnrealInjector struct{}

func (i *UnrealInjector) SupportedEngine() string { return "unreal" }
func (i *UnrealInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "UnrealLocres",
	}
}

func (i *UnrealInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	return &core.InjectionResult{}, nil
}

// ── Godot ───────────────────────────────────────────────────────────

type GodotInjector struct{}

func (i *GodotInjector) SupportedEngine() string { return "godot" }
func (i *GodotInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: true,
		ToolName:          "gdre_tools",
	}
}

func (i *GodotInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	return &core.InjectionResult{}, nil
}

// ── Generic ─────────────────────────────────────────────────────────

type GenericInjector struct{}

func (i *GenericInjector) SupportedEngine() string { return "generic" }
func (i *GenericInjector) Capabilities() core.InjectorCaps {
	return core.InjectorCaps{
		TextInjection:     true,
		NeedsExternalTool: false,
	}
}

func (i *GenericInjector) Inject(ctx context.Context, info *core.GameInfo, proj *core.Project, tools core.ToolRegistry, opts core.PersianOptions) (*core.InjectionResult, error) {
	res := &core.InjectionResult{}
	
	workDir, err := proj.EnsureWorkingDir()
	if err != nil {
		return res, err
	}
	backupMgr := backup.New(info.GameRoot, filepath.Join(filepath.Dir(workDir), "backup"))
	
	translatedEntries := proj.FindTranslated()
	res.StringCount = len(translatedEntries)
	
	// Create backup for all extracted text files
	for _, file := range proj.ExtractedFiles {
		if _, _, err := backupMgr.BackupFile(file); err == nil {
			res.ModifiedFiles = append(res.ModifiedFiles, file)
		}
	}
	
	// Replace text line-by-line (mock implementation for text files)
	
	return res, nil
}
