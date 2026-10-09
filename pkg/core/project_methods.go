package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"farsiforge/pkg/textfilter"
)

// NewProject creates a new project.
func NewProject(name, gameRoot, engine string) *Project {
	return &Project{
		Name:        name,
		GameRoot:    gameRoot,
		Engine:      engine,
		Entries:     []StringEntry{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		PersianOpts: DefaultPersianOptions(),
	}
}

// Save writes the project to a JSON file.
func (p *Project) Save(path string) error {
	p.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write project file: %w", err)
	}
	p.projectFile = path
	return nil
}

// LoadProject reads a project from a JSON file.
func LoadProject(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project file: %w", err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse project file: %w", err)
	}
	p.projectFile = path
	return &p, nil
}

// AddEntry adds a string entry to the project.
func (p *Project) AddEntry(entry StringEntry) {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("s_%04d", len(p.Entries)+1)
	}
	if entry.Status == "" {
		entry.Status = StatusUntranslated
	}
	p.Entries = append(p.Entries, entry)
}

// FindUntranslated returns entries that haven't been translated yet.
func (p *Project) FindUntranslated() []StringEntry {
	var result []StringEntry
	for _, e := range p.Entries {
		if e.Status == StatusUntranslated {
			result = append(result, e)
		}
	}
	return result
}

// FindTranslated returns entries that have been translated.
func (p *Project) FindTranslated() []StringEntry {
	var result []StringEntry
	for _, e := range p.Entries {
		if e.Status == StatusTranslated || e.Status == StatusApproved {
			result = append(result, e)
		}
	}
	return result
}

// Stats returns translation statistics.
func (p *Project) Stats() Stats {
	s := Stats{Total: len(p.Entries)}
	for _, e := range p.Entries {
		switch e.Status {
		case StatusUntranslated:
			s.Untranslated++
		case StatusTranslated:
			s.Translated++
		case StatusApproved:
			s.Approved++
		case StatusSkipped:
			s.Skipped++
		}
	}
	s.Progress = float64(s.Translated+s.Approved) / float64(s.Total) * 100
	if s.Total == 0 {
		s.Progress = 0
	}
	return s
}

// SetTranslation updates the translation for an entry by ID. The entry's
// Notes field records any automatic QA findings (missing placeholders, tag
// mismatches, …) from comparing the translation against its source; an
// empty translation is flagged as EMPTY_TRANSLATION.
func (p *Project) SetTranslation(id, translation string, status Status) error {
	for i := range p.Entries {
		if p.Entries[i].ID == id {
			p.Entries[i].Translation = translation
			p.Entries[i].Status = status
			p.Entries[i].Notes = textfilter.QANotes(textfilter.QA(p.Entries[i].Source, translation))
			p.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// ImportTranslations merges translations from a map of source → translation.
// Each merged entry gets automatic QA notes, same as SetTranslation.
func (p *Project) ImportTranslations(translations map[string]string) int {
	count := 0
	for i := range p.Entries {
		if tr, ok := translations[p.Entries[i].Source]; ok && tr != "" {
			p.Entries[i].Translation = tr
			if p.Entries[i].Status == StatusUntranslated {
				p.Entries[i].Status = StatusTranslated
			}
			p.Entries[i].Notes = textfilter.QANotes(textfilter.QA(p.Entries[i].Source, tr))
			count++
		}
	}
	p.UpdatedAt = time.Now()
	return count
}

// GroupByFile returns entries grouped by their source file.
func (p *Project) GroupByFile() map[string][]StringEntry {
	groups := make(map[string][]StringEntry)
	for _, e := range p.Entries {
		groups[e.File] = append(groups[e.File], e)
	}
	return groups
}

// GroupByContext returns entries grouped by their context.
func (p *Project) GroupByContext() map[string][]StringEntry {
	groups := make(map[string][]StringEntry)
	for _, e := range p.Entries {
		ctx := e.Context
		if ctx == "" {
			ctx = "general"
		}
		groups[ctx] = append(groups[ctx], e)
	}
	return groups
}

// EnsureWorkingDir returns the working directory, creating it if needed.
func (p *Project) EnsureWorkingDir() (string, error) {
	if p.WorkingDir == "" {
		if p.projectFile != "" {
			p.WorkingDir = filepath.Join(filepath.Dir(p.projectFile), "work")
		} else {
			p.WorkingDir = filepath.Join(os.TempDir(), "farsiforge_work")
		}
	}
	if err := os.MkdirAll(p.WorkingDir, 0755); err != nil {
		return "", fmt.Errorf("create working dir: %w", err)
	}
	return p.WorkingDir, nil
}
