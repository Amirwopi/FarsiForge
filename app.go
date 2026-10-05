package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"farsiforge/pkg/core"
	"farsiforge/pkg/detection"
	"farsiforge/pkg/extract"
	"farsiforge/pkg/inject"
	"farsiforge/pkg/tools"
)

// App struct
type App struct {
	ctx      context.Context
	registry *detection.Registry
	toolReg  *tools.Registry
}

// NewApp creates a new App application struct
func NewApp() *App {
	toolReg, _ := tools.NewRegistry("Tools")
	return &App{
		registry: detection.DefaultRegistry(),
		toolReg:  toolReg,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// DetectEngine runs the engine detection.
func (a *App) DetectEngine(path string) (*core.GameInfo, error) {
	res, err := a.registry.Detect(path)
	if err != nil {
		return nil, fmt.Errorf("detection failed: %v", err)
	}

	dataPath := ""
	if len(res.DataPaths) > 0 {
		dataPath = res.DataPaths[0]
	}

	return &core.GameInfo{
		Engine:     res.Engine,
		Backend:    res.Backend,
		Version:    res.Version,
		GameName:   res.GameName,
		GameExe:    res.GameExe,
		GameRoot:   path,
		DataPath:   dataPath,
		Confidence: res.Confidence,
		Evidence:   res.Evidence,
		Metadata:   res.Metadata,
	}, nil
}

// Extract runs the extraction pipeline.
func (a *App) Extract(info *core.GameInfo) (int, error) {
	proj := core.NewProject("Local Project", info.GameRoot, info.Engine)
	
	err := extract.Run(a.ctx, info, proj, a.toolReg)
	if err != nil {
		return 0, fmt.Errorf("extraction failed: %v", err)
	}
	
	proj.Save(info.GameRoot + "/.farsiforge_project.json")
	
	return len(proj.Entries), nil
}

// Inject runs the injection pipeline.
func (a *App) Inject(info *core.GameInfo) (int, error) {
	proj, err := core.LoadProject(info.GameRoot + "/.farsiforge_project.json")
	if err != nil {
		proj = core.NewProject("Mock Project", info.GameRoot, info.Engine)
	}
	
	opts := core.DefaultPersianOptions()
	
	modified, err := inject.Run(a.ctx, info, proj, a.toolReg, opts)
	if err != nil {
		return 0, fmt.Errorf("injection failed: %v", err)
	}
	
	return len(modified), nil
}

// LaunchGame starts the game executable
func (a *App) LaunchGame(gameRoot, exeName string) error {
	exePath := filepath.Join(gameRoot, exeName)
	cmd := exec.Command(exePath)
	cmd.Dir = gameRoot
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch game: %v", err)
	}
	return nil
}

