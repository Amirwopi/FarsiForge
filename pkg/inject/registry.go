package inject

import (
	"context"
	"fmt"

	"farsiforge/pkg/core"
	"farsiforge/pkg/logging"
)

var log = logging.Default().WithModule("inject")

// Registry holds all registered injectors.
type Registry struct {
	injectors map[string]core.IInjector
}

// NewRegistry creates a new empty injector registry.
func NewRegistry() *Registry {
	return &Registry{
		injectors: make(map[string]core.IInjector),
	}
}

// Register adds an injector to the registry.
func (r *Registry) Register(injector core.IInjector) {
	r.injectors[injector.SupportedEngine()] = injector
	log.Debug("Registered injector", "engine", injector.SupportedEngine())
}

// GetInjector returns the appropriate injector for the detected engine.
func (r *Registry) GetInjector(engine string) (core.IInjector, error) {
	if inj, ok := r.injectors[engine]; ok {
		return inj, nil
	}
	
	// Fallback
	if inj, ok := r.injectors["generic"]; ok {
		return inj, nil
	}
	
	return nil, fmt.Errorf("no injector available for engine: %s", engine)
}

// DefaultRegistry creates and returns a registry populated with all
// built-in engine injectors.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	
	r.Register(&UnityInjector{})
	r.Register(&UnrealInjector{})
	r.Register(&GodotInjector{})
	r.Register(&GenericInjector{})
	
	return r
}

// Run executes the full injection pipeline.
func Run(ctx context.Context, info *core.GameInfo, proj *core.Project, reg core.ToolRegistry, opts core.PersianOptions) ([]string, error) {
	r := DefaultRegistry()
	
	inj, err := r.GetInjector(info.Engine)
	if err != nil {
		return nil, core.Wrap("inject", err, "could not find injector")
	}

	// Check if external tools are needed and available
	caps := inj.Capabilities()
	if caps.NeedsExternalTool && caps.ToolName != "" {
		if !reg.IsAvailable(caps.ToolName) {
			return nil, core.ErrToolNotFound(caps.ToolName)
		}
	}

	log.Info("Starting injection", "engine", info.Engine, "game", proj.GameName)
	
	result, err := inj.Inject(ctx, info, proj, reg, opts)
	if err != nil {
		return nil, core.ErrInjectionFailed(info.Engine, err)
	}
	
	log.Info("Injection completed", "modified_files", len(result.ModifiedFiles), "strings_injected", result.StringCount)
	
	proj.ModifiedFiles = result.ModifiedFiles
	return result.ModifiedFiles, nil
}
