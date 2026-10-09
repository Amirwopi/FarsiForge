package main

import (
	"context"
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
	action := flag.String("action", "detect", "Action to perform (detect, extract, pipeline, search)")
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
		projPath := filepath.Join(*gameDir, ".farsiforge_project.json")
		proj, err := core.LoadProject(projPath)
		if err != nil {
			fmt.Printf("No project file found at %s (run -action extract first): %v\n", projPath, err)
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
		Confidence: res.Confidence,
	}

	proj := core.NewProject(info.GameName+" Localization", info.GameRoot, res.Engine)

	if *action == "extract" || *action == "pipeline" {
		fmt.Println("\nStarting Extraction...")
		if err := extract.Run(ctx, info, proj, toolReg); err != nil {
			fmt.Printf("Extraction failed: %v\n", err)
			os.Exit(1)
		}
		stats := proj.Stats()
		fmt.Printf("Extraction successful! Found %d strings in %d files.\n", stats.Total, len(proj.ExtractedFiles))
	}

	if *action == "pipeline" {
		fmt.Println("\nStarting Injection (Dry Run)...")

		// Add mock translations for the sake of pipeline testing
		count := 0
		for _, entry := range proj.FindUntranslated() {
			if count > 10 {
				break
			}
			proj.SetTranslation(entry.ID, "[PERSIAN] "+entry.Source, core.StatusTranslated)
			count++
		}

		opts := core.DefaultPersianOptions()
		modifiedFiles, err := inject.Run(ctx, info, proj, toolReg, opts)
		if err != nil {
			fmt.Printf("Injection failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Injection successful! Modified %d files.\n", len(modifiedFiles))
	}
}
