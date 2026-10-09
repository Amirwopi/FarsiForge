package inject

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	if info == nil || proj == nil {
		return nil, core.NewError("inject", "game information and project are required")
	}
	// Invalidate the previous staging checkpoint before any output is written.
	// A failed re-injection must never leave an older patch buildable by accident.
	proj.ModifiedFiles = nil
	proj.ModifiedFileHashes = nil
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
	if result == nil {
		return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("injector returned no result"))
	}
	if len(result.Errors) > 0 {
		return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("injection incomplete: %s", strings.Join(result.Errors, "; ")))
	}
	if len(result.ModifiedFiles) == 0 {
		return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("no staged files were produced"))
	}
	hashes := make(map[string]string, len(result.ModifiedFiles))
	for _, relPath := range result.ModifiedFiles {
		cleanPath, err := cleanGameRelativePath(relPath)
		if err != nil {
			return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("invalid staged target %q: %w", relPath, err))
		}
		sourcePath, err := gameFilePath(info.GameRoot, cleanPath)
		if err != nil {
			return nil, core.ErrInjectionFailed(info.Engine, err)
		}
		file, err := os.Open(sourcePath)
		if err != nil {
			return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("open original target %s: %w", relPath, err))
		}
		hasher := sha256.New()
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("hash original target %s: %w", relPath, copyErr))
		}
		if closeErr != nil {
			return nil, core.ErrInjectionFailed(info.Engine, fmt.Errorf("close original target %s: %w", relPath, closeErr))
		}
		hashes[filepath.ToSlash(cleanPath)] = fmt.Sprintf("%x", hasher.Sum(nil))
	}
	log.Info("Injection completed", "modified_files", len(result.ModifiedFiles), "strings_injected", result.StringCount)

	proj.ModifiedFiles = result.ModifiedFiles
	proj.ModifiedFileHashes = hashes
	return result.ModifiedFiles, nil
}
