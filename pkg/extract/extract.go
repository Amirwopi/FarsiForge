package extract

import (
	"fmt"

	"farsiforge/pkg/detection"
	"farsiforge/pkg/project"
	"farsiforge/pkg/tools"
)

// Extractor is the interface each engine extractor implements.
type Extractor interface {
	// Extract pulls translatable strings from the game and populates
	// the project with StringEntry values.
	Extract(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error

	// Capabilities returns what this extractor can do.
	Capabilities() Capabilities
}

// Capabilities describes what an extractor/injector can handle.
type Capabilities struct {
	TextExtraction  bool
	DialogueExtraction bool
	FontExtraction  bool
	FontInjection   bool
	AssetExtraction bool
	NeedsExternalTool bool
	ToolName        string
}

// GetExtractor returns the appropriate extractor for the detected engine.
func GetExtractor(engine detection.Engine) (Extractor, error) {
	switch engine {
	case detection.EngineUnity:
		return &UnityExtractor{}, nil
	case detection.EngineUnreal:
		return &UnrealExtractor{}, nil
	case detection.EngineGodot:
		return &GodotExtractor{}, nil
	case detection.EngineRPGMaker:
		return &RPGMakerExtractor{}, nil
	case detection.EngineGameMaker:
		return &GameMakerExtractor{}, nil
	case detection.EngineRenPy:
		return &RenPyExtractor{}, nil
	case detection.EngineSource, detection.EngineGoldSrc:
		return &SourceExtractor{}, nil
	case detection.EngineAdobeAIR:
		return &AdobeAIRExtractor{}, nil
	case detection.EngineCustom:
		return &TextFileExtractor{}, nil
	default:
		return &TextFileExtractor{}, nil
	}
}

// Run executes the full extraction pipeline.
func Run(info *detection.GameInfo, proj *project.Project, reg *tools.Registry) error {
	ext, err := GetExtractor(info.Engine)
	if err != nil {
		return fmt.Errorf("no extractor for engine %s: %w", info.Engine, err)
	}

	// Check if external tools are needed and available
	caps := ext.Capabilities()
	if caps.NeedsExternalTool && caps.ToolName != "" {
		if !reg.IsAvailable(caps.ToolName) {
			return fmt.Errorf("required tool not available: %s. Please install or extract it in the Tools directory", caps.ToolName)
		}
	}

	return ext.Extract(info, proj, reg)
}
