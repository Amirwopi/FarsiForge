package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"farsiforge/pkg/core"
	"farsiforge/pkg/detection"
	"farsiforge/pkg/extract"
	"farsiforge/pkg/inject"
	"farsiforge/pkg/tools"
)

func main() {
	gameDir := flag.String("dir", "", "Path to the game directory")
	action := flag.String("action", "detect", "Action to perform (detect, extract, inject, pipeline, search)")
	query := flag.String("query", "", "Search query for the search action (matches ID, source, translation, file, notes)")
	status := flag.String("status", "", "Optional status filter for search (untranslated, translated, approved, skipped, qa)")
	flag.Parse()

	if *gameDir == "" {
		fmt.Println("Error: --dir is required")
		flag.Usage()
		os.Exit(1)
	}

	// Search works on a previously saved project and needs no detection.
	if *action == "search" {
		proj, err := core.LoadGameProject(*gameDir)
		if err != nil {
			fmt.Printf("No project file found outside the game directory (run -action extract first): %v\n", err)
			os.Exit(1)
		}
		matches := proj.SearchEntries(*query, *status)
		fmt.Printf("Found %d matching entries:\n", len(matches))
		for i, e := range matches {
			if i >= 50 {
				fmt.Println("  ... (showing first 50; refine the query to narrow down)")
				break
			}
			fmt.Printf("  [%s] %s\n", e.ID, e.Source)
			if e.Translation != "" {
				fmt.Printf("        → %s\n", e.Translation)
			}
			if e.Notes != "" {
				fmt.Printf("        ⚠ %s\n", e.Notes)
			}
		}
		return
	}

	ctx := context.Background()
	reg := detection.DefaultRegistry()
	toolReg, err := tools.NewRegistry("")
	if err != nil {
		fmt.Printf("Warning: Tools registry init failed: %v\n", err)
	}

	fmt.Printf("Analyzing game at: %s\n", *gameDir)
	res, err := reg.Detect(*gameDir)
	if err != nil {
		fmt.Printf("Detection error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("──────────────────────────────────────────")
	fmt.Printf("Engine     : %s\n", res.Engine)
	fmt.Printf("Confidence : %.0f%%\n", res.Confidence*100)
	fmt.Printf("Backend    : %s\n", res.Backend)
	fmt.Printf("Version    : %s\n", res.Version)
	fmt.Println("Evidence   :")
	for _, ev := range res.Evidence {
		fmt.Printf("  - %s\n", ev)
	}
	fmt.Println("──────────────────────────────────────────")

	if *action == "detect" {
		return
	}

	info := &core.GameInfo{
		Engine:     res.Engine,
		Backend:    res.Backend,
		Version:    res.Version,
		GameName:   filepath.Base(*gameDir),
		GameExe:    res.GameExe,
		GameRoot:   *gameDir,
		DataPath:   firstDataPath(res.DataPaths),
		Metadata:   res.Metadata,
		Confidence: res.Confidence,
	}
	info.Backend = res.Backend
	info.Version = res.Version

	if *action == "inject" {
		proj, err := core.LoadGameProject(info.GameRoot)
		if err != nil {
			fmt.Printf("Load project failed: %v\n", err)
			os.Exit(1)
		}
		if proj.Engine != info.Engine {
			fmt.Printf("Project engine %q does not match detected engine %q\n", proj.Engine, info.Engine)
			os.Exit(1)
		}
		proj.ModifiedFiles = nil
		proj.ModifiedFileHashes = nil
		projectPath, err := core.ProjectFilePath(info.GameRoot)
		if err != nil {
			fmt.Printf("Failed to resolve project path: %v\n", err)
			os.Exit(1)
		}
		if err := proj.Save(projectPath); err != nil {
			fmt.Printf("Failed to clear previous staged injection state: %v\n", err)
			os.Exit(1)
		}
		opts := proj.PersianOpts
		if opts == (core.PersianOptions{}) {
			opts = core.DefaultPersianOptions()
		}
		modified, err := inject.Run(ctx, info, proj, toolReg, opts)
		if err != nil {
			fmt.Printf("Injection failed: %v\n", err)
			os.Exit(1)
		}
		if err := proj.Save(projectPath); err != nil {
			fmt.Printf("Failed to save injection project state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Injection staged %d files outside the game directory.\n", len(modified))
		return
	}

	var previous *core.Project
	if existing, loadErr := core.LoadGameProject(info.GameRoot); loadErr == nil {
		previous = existing
	} else if !errors.Is(loadErr, os.ErrNotExist) {
		fmt.Printf("Load existing project failed: %v\n", loadErr)
		os.Exit(1)
	}
	proj, projectPath, err := core.NewGameProject(info.GameName+" Localization", info.GameRoot, res.Engine)
	if err != nil {
		fmt.Printf("Create project failed: %v\n", err)
		os.Exit(1)
	}
	proj.GameName = info.GameName
	proj.GameExe = info.GameExe
	proj.Backend = info.Backend
	proj.Version = info.Version

	if *action == "extract" || *action == "pipeline" {
		fmt.Println("\nStarting Extraction...")
		if err := extract.Run(ctx, info, proj, toolReg); err != nil {
			fmt.Printf("Extraction failed: %v\n", err)
			os.Exit(1)
		}
		proj.MergeTranslations(previous)
		if err := proj.Save(projectPath); err != nil {
			fmt.Printf("Saving extracted project failed: %v\n", err)
			os.Exit(1)
		}
		stats := proj.Stats()
		fmt.Printf("Extraction successful! Found %d strings in %d files.\nProject: %s\n", stats.Total, len(proj.ExtractedFiles), projectPath)
	}

	if *action == "pipeline" {
		if len(proj.FindTranslated()) == 0 {
			fmt.Println("No saved translations are available; extraction completed without injection.")
			return
		}
		modifiedFiles, err := inject.Run(ctx, info, proj, toolReg, proj.PersianOpts)
		if err != nil {
			fmt.Printf("Injection failed: %v\n", err)
			os.Exit(1)
		}
		if err := proj.Save(projectPath); err != nil {
			fmt.Printf("Saving injection results failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Injection staged %d files outside the game directory.\n", len(modifiedFiles))
	}
}

func firstDataPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}
