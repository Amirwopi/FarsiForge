package extract

import (
	"context"
	"fmt"

	"farsiforge/pkg/core"
	"farsiforge/pkg/logging"
)

var log = logging.Default().WithModule("extract")

// Registry holds all registered extractors.
type Registry struct {
	extractors map[string]core.IExtractor
}

// NewRegistry creates a new empty extractor registry.
func NewRegistry() *Registry {
	return &Registry{
		extractors: make(map[string]core.IExtractor),
	}
}

// Register adds an extractor to the registry.
func (r *Registry) Register(extractor core.IExtractor) {
	r.extractors[extractor.SupportedEngine()] = extractor
	log.Debug("Registered extractor", "engine", extractor.SupportedEngine())
}

// GetExtractor returns the appropriate extractor for the detected engine.
func (r *Registry) GetExtractor(engine string) (core.IExtractor, error) {
	if ext, ok := r.extractors[engine]; ok {
		return ext, nil
	}

	// Fallback
	if ext, ok := r.extractors["generic"]; ok {
		return ext, nil
	}

	return nil, fmt.Errorf("no extractor available for engine: %s", engine)
}

// DefaultRegistry creates and returns a registry populated with all
// built-in engine extractors.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register(&UnityExtractor{})
	r.Register(&UnrealExtractor{})
	r.Register(&GodotExtractor{})
	r.Register(&GoldSrcExtractor{})
	r.Register(&Source2Extractor{})
	r.Register(&FactorioExtractor{})
	r.Register(&ZomboidExtractor{})
	r.Register(&SAGEExtractor{})
	r.Register(&RAGEExtractor{})
	r.Register(&FromSoftwareExtractor{})
	r.Register(&GenericExtractor{})

	// RPGMaker, GameMaker, RenPy, Source, AdobeAIR can be added here

	return r
}

// Run executes the full extraction pipeline.
func Run(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry) error {
	r := DefaultRegistry()

	ext, err := r.GetExtractor(info.Engine)
	if err != nil {
		return core.Wrap("extract", err, "could not find extractor")
	}

	// Check if external tools are needed and available
	caps := ext.Capabilities()
	if caps.NeedsExternalTool && caps.ToolName != "" {
		if !reg.IsAvailable(caps.ToolName) {
			return core.ErrToolNotFound(caps.ToolName)
		}
	}

	log.Info("Starting extraction", "engine", info.Engine, "game", proj.GameName)

	if err := ext.Extract(ctx, info, proj, reg); err != nil {
		return core.ErrExtractionFailed(info.Engine, err)
	}

	stats := proj.Stats()
	log.Info("Extraction completed", "entries", stats.Total)

	return nil
}
