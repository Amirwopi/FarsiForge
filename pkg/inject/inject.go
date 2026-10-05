package inject

import (
	"fmt"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/persian"
	"farsiforge/pkg/project"
	"farsiforge/pkg/tools"
)

// Injector is the interface each engine injector implements.
type Injector interface {
	// Inject writes translated strings back into game files.
	Inject(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error)

	// Capabilities returns what this injector can do.
	Capabilities() Capabilities
}

// Capabilities describes injector capabilities.
type Capabilities struct {
	TextInjection   bool
	FontInjection   bool
	NeedsExternalTool bool
	ToolName        string
}

// GetInjector returns the appropriate injector for the detected engine.
func GetInjector(engine detection.Engine) (Injector, error) {
	switch engine {
	case detection.EngineUnity:
		return &UnityInjector{}, nil
	case detection.EngineUnreal:
		return &UnrealInjector{}, nil
	case detection.EngineGodot:
		return &GodotInjector{}, nil
	case detection.EngineRPGMaker:
		return &RPGMakerInjector{}, nil
	case detection.EngineRenPy:
		return &RenPyInjector{}, nil
	case detection.EngineSource, detection.EngineGoldSrc:
		return &SourceInjector{}, nil
	case detection.EngineAdobeAIR:
		return &AdobeAIRInjector{}, nil
	case detection.EngineCustom:
		return &TextFileInjector{}, nil
	default:
		return &TextFileInjector{}, nil
	}
}

// Run executes the full injection pipeline.
// Returns a list of modified file paths.
func Run(info *detection.GameInfo, proj *project.Project, reg *tools.Registry, opts persian.Options) ([]string, error) {
	inj, err := GetInjector(info.Engine)
	if err != nil {
		return nil, fmt.Errorf("no injector for engine %s: %w", info.Engine, err)
	}

	caps := inj.Capabilities()
	if caps.NeedsExternalTool && caps.ToolName != "" {
		if !reg.IsAvailable(caps.ToolName) {
			return nil, fmt.Errorf("required tool not available: %s", caps.ToolName)
		}
	}

	return inj.Inject(info, proj, reg, opts)
}

// ProcessEntry applies Persian text processing to a translation.
func ProcessEntry(entry project.StringEntry, opts persian.Options) string {
	return persian.Process(entry.Translation, opts)
}

// BuildTranslationMap creates a map of source → processed translation
// for all translated entries.
func BuildTranslationMap(proj *project.Project, opts persian.Options) map[string]string {
	m := make(map[string]string)
	for _, e := range proj.Entries {
		if e.Status == project.StatusTranslated || e.Status == project.StatusApproved {
			if e.Translation != "" {
				processed := persian.Process(e.Translation, opts)
				m[e.Source] = processed
			}
		}
	}
	return m
}
