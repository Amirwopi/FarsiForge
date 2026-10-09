package extract

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"farsiforge/pkg/core"
	"farsiforge/pkg/scanner"
)

// RAGEExtractor reads standalone GXT2 tables and reports encrypted RPF
// archives that need executable-derived crypto material.
type RAGEExtractor struct{}

func (*RAGEExtractor) SupportedEngine() string { return "rage" }
func (*RAGEExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true}
}
func (*RAGEExtractor) Extract(_ context.Context, info *core.GameInfo, proj *core.Project, _ core.ToolRegistry) error {
	files := scanner.WalkDir(info.GameRoot, 12, func(p string) bool { return strings.EqualFold(filepath.Ext(p), ".gxt2") })
	for _, p := range files {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		rows, e := ParseGXT2(b)
		if e != nil {
			log.Warn("invalid GXT2", "file", p, "error", e)
			continue
		}
		rel, _ := filepath.Rel(info.GameRoot, p)
		for _, row := range rows {
			if strings.TrimSpace(row.Text) == "" {
				continue
			}
			proj.AddEntry(core.StringEntry{Source: row.Text, File: filepath.ToSlash(rel), Path: fmt.Sprintf("0x%08x", row.Hash), Context: "RAGE GXT2"})
		}
		proj.ExtractedFiles = append(proj.ExtractedFiles, filepath.ToSlash(rel))
	}
	archives := scanner.WalkDir(info.GameRoot, 8, func(p string) bool { return strings.EqualFold(filepath.Ext(p), ".rpf") })
	var ng int
	for _, p := range archives {
		f, e := os.Open(p)
		if e != nil {
			continue
		}
		var h [16]byte
		_, e = f.Read(h[:])
		f.Close()
		if e == nil && binary.LittleEndian.Uint32(h[:4]) == 0x52504637 && binary.LittleEndian.Uint32(h[12:16]) == 0x0fefffff {
			ng++
		}
	}
	if ng > 0 && len(files) == 0 {
		return fmt.Errorf("found %d RPF7 NG-encrypted archives but cannot derive their NG tables without the matching game executable key hashes", ng)
	}
	return nil
}

// FromSoftwareExtractor extracts loose FMG files and validates encrypted BHD5
// headers; supported BND/DCX wrappers are handled by the parsers in this package.
type FromSoftwareExtractor struct{}

func (*FromSoftwareExtractor) SupportedEngine() string { return "fromsoftware" }
func (*FromSoftwareExtractor) Capabilities() core.ExtractorCaps {
	return core.ExtractorCaps{TextExtraction: true}
}
func (*FromSoftwareExtractor) Extract(_ context.Context, info *core.GameInfo, proj *core.Project, _ core.ToolRegistry) error {
	files := scanner.WalkDir(info.GameRoot, 12, func(p string) bool { return strings.EqualFold(filepath.Ext(p), ".fmg") })
	for _, p := range files {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		rows, e := ParseFMG(b)
		if e != nil {
			log.Warn("invalid FMG", "file", p, "error", e)
			continue
		}
		rel, _ := filepath.Rel(info.GameRoot, p)
		for _, row := range rows {
			if strings.TrimSpace(row.Text) != "" {
				proj.AddEntry(core.StringEntry{Source: row.Text, File: filepath.ToSlash(rel), Path: fmt.Sprint(row.ID), Context: "FromSoftware FMG"})
			}
		}
		proj.ExtractedFiles = append(proj.ExtractedFiles, filepath.ToSlash(rel))
	}
	if len(files) > 0 {
		return nil
	}
	// The family reader currently has the ER Data0 public key. Other archive
	// versions and keys are selected only when the file's variant is known.
	bhdPath := filepath.Join(info.GameRoot, "Data0.bhd")
	if raw, err := os.ReadFile(bhdPath); err == nil {
		idx, err := DecryptERData0BHD(raw)
		if err != nil {
			return fmt.Errorf("decrypt Data0.bhd: %w", err)
		}
		log.Info("Parsed FromSoftware BHD5 index", "file", bhdPath, "entries", len(idx.Files))
		bdtPath := filepath.Join(info.GameRoot, "Data0.bdt")
		bdt, err := os.Open(bdtPath)
		if err != nil {
			return fmt.Errorf("open Data0.bdt: %w", err)
		}
		defer bdt.Close()
		byHash := make(map[uint64]ERBHDFile, len(idx.Files))
		for _, f := range idx.Files {
			byHash[f.Hash] = f
		}
		candidates := make([]string, 0, 8)
		for _, lang := range []string{"engus", "jpn", "deude", "frafr", "ita", "spaes", "rus", "pol", "porbr", "kor", "zhocn"} {
			for _, base := range []string{"item", "menu", "item_dlc01", "menu_dlc01", "system", "talk"} {
				candidates = append(candidates, "/msg/"+lang+"/"+base+".msgbnd.dcx")
			}
		}
		sort.Strings(candidates)
		seen := map[uint64]bool{}
		for _, name := range candidates {
			h := ERPathHash(name)
			f, ok := byHash[h]
			if !ok || seen[h] {
				continue
			}
			seen[h] = true
			dcx, err := ReadERBHDFile(idx, bdt, f)
			if err != nil {
				log.Warn("read FromSoftware BDT entry", "path", name, "error", err)
				continue
			}
			decoded, err := DecompressDCXWithOodle(dcx, filepath.Join(info.GameRoot, "oo2core_6_win64.dll"))
			if err != nil {
				log.Warn("decompress FromSoftware DCX", "path", name, "error", err)
				continue
			}
			files, err := ParseBND4(decoded)
			if err != nil {
				log.Warn("parse FromSoftware BND4", "path", name, "error", err)
				continue
			}
			for _, file := range files {
				if !isFMGName(file.Name) {
					continue
				}
				rows, err := ParseFMG(file.Data)
				if err != nil {
					log.Warn("parse FromSoftware FMG", "file", file.Name, "error", err)
					continue
				}
				for _, row := range rows {
					if strings.TrimSpace(row.Text) != "" {
						proj.AddEntry(core.StringEntry{Source: row.Text, File: name + "/" + file.Name, Path: fmt.Sprint(row.ID), Context: "FromSoftware FMG"})
					}
				}
			}
			if len(files) > 0 {
				proj.ExtractedFiles = append(proj.ExtractedFiles, name)
			}
		}
		if len(proj.Entries) > 0 {
			return nil
		}
		return fmt.Errorf("Data0.bhd index parsed (%d entries), but no candidate ER message archive yielded FMG strings", len(idx.Files))
	}
	return fmt.Errorf("no loose FMG files or supported BHD5 archive found")
}
