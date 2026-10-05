package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status represents the translation status of a string entry.
type Status string

const (
	StatusUntranslated Status = "untranslated"
	StatusTranslated   Status = "translated"
	StatusApproved     Status = "approved"
	StatusSkipped      Status = "skipped"
)

// StringEntry represents one translatable string extracted from a game.
type StringEntry struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Translation string `json:"translation"`
	File        string `json:"file"`       // source file path (relative to game root)
	Path        string `json:"path"`       // internal path within the file (e.g. asset path, JSON key)
	Context     string `json:"context"`    // additional context (e.g. "dialogue", "UI", "item name")
	Status      Status `json:"status"`
	Character   string `json:"character"`  // speaking character (for dialogue)
	Notes       string `json:"notes"`      // translator notes
}

// Project represents a FarsiForge localization project.
type Project struct {
	// Metadata
	Name        string    `json:"name"`
	GameName    string    `json:"game_name"`
	GameRoot    string    `json:"game_root"`
	Engine      string    `json:"engine"`
	Backend     string    `json:"backend"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// State
	Entries     []StringEntry `json:"entries"`
	WorkingDir  string        `json:"working_dir"`  // extraction/injection working directory

	// Settings
	PersianOpts  map[string]bool `json:"persian_opts"`
	FontPath     string          `json:"font_path"`    // path to Persian font for injection
	InstallerName string         `json:"installer_name"`

	// File tracking
	ExtractedFiles []string `json:"extracted_files"` // files extracted from game
	ModifiedFiles  []string `json:"modified_files"`  // files to inject back

	// Internal
	projectFile string `json:"-"`
}

// New creates a new project.
func New(name, gameRoot, engine string) *Project {
	return &Project{
		Name:      name,
		GameRoot:  gameRoot,
		Engine:    engine,
		Entries:   []StringEntry{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		PersianOpts: map[string]bool{
			"reshape":       true,
			"bidi_reorder":  true,
			"fix_yeh":       true,
			"persian_digits": true,
		},
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

// Load reads a project from a JSON file.
func Load(path string) (*Project, error) {
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

// Stats holds translation progress statistics.
type Stats struct {
	Total        int     `json:"total"`
	Untranslated int     `json:"untranslated"`
	Translated   int     `json:"translated"`
	Approved     int     `json:"approved"`
	Skipped      int     `json:"skipped"`
	Progress     float64 `json:"progress"`
}

// SetTranslation updates the translation for an entry by ID.
func (p *Project) SetTranslation(id, translation string, status Status) error {
	for i := range p.Entries {
		if p.Entries[i].ID == id {
			p.Entries[i].Translation = translation
			p.Entries[i].Status = status
			p.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("entry not found: %s", id)
}

// ImportTranslations merges translations from a map of ID → translation.
func (p *Project) ImportTranslations(translations map[string]string) int {
	count := 0
	for i := range p.Entries {
		if tr, ok := translations[p.Entries[i].ID]; ok && tr != "" {
			p.Entries[i].Translation = tr
			if p.Entries[i].Status == StatusUntranslated {
				p.Entries[i].Status = StatusTranslated
			}
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

// WorkingDir returns the working directory, creating it if needed.
func (p *Project) EnsureWorkingDir() (string, error) {
	if p.WorkingDir == "" {
		p.WorkingDir = filepath.Join(filepath.Dir(p.projectFile), "work")
	}
	if err := os.MkdirAll(p.WorkingDir, 0755); err != nil {
		return "", fmt.Errorf("create working dir: %w", err)
	}
	return p.WorkingDir, nil
}
