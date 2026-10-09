// Package core defines the plugin interfaces and shared types for FarsiForge.
//
// Every engine-specific module (detector, extractor, injector) implements
// the interfaces defined here. The pipeline orchestrator uses these
// interfaces to run the localization workflow without knowing engine details.
package core

import (
	"context"
	"time"
)

// ─── Engine Detection ──────────────────────────────────────────────

// IEngineDetector is the interface every engine detector must implement.
// Detectors are registered in a DetectorRegistry and called in priority order.
type IEngineDetector interface {
	// Detect examines a game directory and returns detection results.
	// Returns nil, nil if this detector doesn't recognize the game.
	Detect(gameDir string) (*DetectionResult, error)

	// Name returns the detector's engine name (e.g., "unity", "unreal").
	Name() string

	// Priority returns the detection priority. Higher = checked first.
	// Use 100 for specific engines, 50 for generic, 0 for fallback.
	Priority() int
}

// DetectionResult holds the output of engine detection.
type DetectionResult struct {
	Engine     string            `json:"engine"`
	Version    string            `json:"version"`
	Backend    string            `json:"backend,omitempty"`
	Confidence float64           `json:"confidence"`
	Evidence   []string          `json:"evidence"`
	DataPaths  []string          `json:"data_paths,omitempty"`
	GameExe    string            `json:"game_exe,omitempty"`
	GameName   string            `json:"game_name,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ─── Extraction ────────────────────────────────────────────────────

// IExtractor is the interface for text extraction from game files.
type IExtractor interface {
	// Extract pulls translatable strings from the game and adds them to the project.
	Extract(ctx context.Context, info *GameInfo, proj *Project, tools ToolRegistry) error

	// Capabilities returns what this extractor can do.
	Capabilities() ExtractorCaps

	// SupportedEngine returns which engine this extractor handles.
	SupportedEngine() string
}

// ExtractorCaps describes extractor capabilities.
type ExtractorCaps struct {
	TextExtraction     bool   `json:"text_extraction"`
	DialogueExtraction bool   `json:"dialogue_extraction"`
	FontExtraction     bool   `json:"font_extraction"`
	AssetExtraction    bool   `json:"asset_extraction"`
	NeedsExternalTool  bool   `json:"needs_external_tool"`
	ToolName           string `json:"tool_name,omitempty"`
}

// ─── Injection ─────────────────────────────────────────────────────

// IInjector is the interface for writing translations back into game files.
type IInjector interface {
	// Inject writes translated strings back into game files.
	// Returns a list of modified files and injection statistics.
	Inject(ctx context.Context, info *GameInfo, proj *Project, tools ToolRegistry, opts PersianOptions) (*InjectionResult, error)

	// Capabilities returns what this injector can do.
	Capabilities() InjectorCaps

	// SupportedEngine returns which engine this injector handles.
	SupportedEngine() string
}

// InjectorCaps describes injector capabilities.
type InjectorCaps struct {
	TextInjection     bool   `json:"text_injection"`
	FontInjection     bool   `json:"font_injection"`
	NeedsExternalTool bool   `json:"needs_external_tool"`
	ToolName          string `json:"tool_name,omitempty"`
}

// InjectionResult holds the output of an injection operation.
type InjectionResult struct {
	ModifiedFiles []string `json:"modified_files"`
	BackupFiles   []string `json:"backup_files"`
	StringCount   int      `json:"string_count"`
	Errors        []string `json:"errors,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// ─── Validation ────────────────────────────────────────────────────

// IValidator validates translations before injection.
type IValidator interface {
	// Validate checks entries and returns validation errors.
	Validate(entries []StringEntry) []ValidationError
}

// ValidationError describes a single validation failure.
type ValidationError struct {
	EntryID  string         `json:"entry_id"`
	Type     ValidationType `json:"type"`
	Message  string         `json:"message"`
	Severity Severity       `json:"severity"`
	Details  string         `json:"details,omitempty"`
}

// ValidationType identifies what kind of validation failed.
type ValidationType string

const (
	ValPlaceholderMismatch ValidationType = "placeholder_mismatch"
	ValEncodingError       ValidationType = "encoding_error"
	ValEmptyTranslation    ValidationType = "empty_translation"
	ValLengthExceeded      ValidationType = "length_exceeded"
	ValEscapeError         ValidationType = "escape_error"
	ValMarkupError         ValidationType = "markup_error"
)

// Severity levels for validation errors.
type Severity string

const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
	SevInfo    Severity = "info"
)

// ─── Shared Data Types ─────────────────────────────────────────────

// GameInfo holds all detected information about a game.
type GameInfo struct {
	Engine     string            `json:"engine"`
	Backend    string            `json:"backend,omitempty"`
	Version    string            `json:"version,omitempty"`
	GameName   string            `json:"game_name"`
	GameExe    string            `json:"game_exe,omitempty"`
	GameRoot   string            `json:"game_root"`
	DataPath   string            `json:"data_path,omitempty"`
	Platform   string            `json:"platform,omitempty"`
	Arch       string            `json:"arch,omitempty"`
	Confidence float64           `json:"confidence"`
	Evidence   []string          `json:"evidence,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

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
	Translation string `json:"translation,omitempty"`
	File        string `json:"file"`
	// Container is the game-relative archive/pack path that contains File.
	// It is empty when the source is a loose game file.
	Container string `json:"container,omitempty"`
	Path      string `json:"path"`
	Context   string `json:"context,omitempty"`
	Speaker   string `json:"speaker,omitempty"`
	Line      int    `json:"line,omitempty"`
	Status    Status `json:"status"`
	Notes     string `json:"notes,omitempty"`
	MaxLength int    `json:"max_length,omitempty"`
}

// PersianOptions controls how Persian text is processed for injection.
type PersianOptions struct {
	Reshape        bool `json:"reshape"`
	BidiReorder    bool `json:"bidi_reorder"`
	FixYeh         bool `json:"fix_yeh"`
	PersianDigits  bool `json:"persian_digits"`
	ConvertPunct   bool `json:"convert_punct"`
	DropDiacritics bool `json:"drop_diacritics"`
}

// DefaultPersianOptions returns the standard processing options.
func DefaultPersianOptions() PersianOptions {
	return PersianOptions{
		Reshape:       true,
		BidiReorder:   true,
		FixYeh:        true,
		PersianDigits: true,
	}
}

// ─── Project ───────────────────────────────────────────────────────

// Project represents a FarsiForge localization project.
type Project struct {
	// Metadata
	Name      string    `json:"name"`
	GameName  string    `json:"game_name"`
	GameRoot  string    `json:"game_root"`
	Engine    string    `json:"engine"`
	GameExe   string    `json:"game_exe,omitempty"`
	Backend   string    `json:"backend,omitempty"`
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Entries
	Entries []StringEntry `json:"entries"`

	// Working directories
	WorkingDir string `json:"working_dir,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`

	// Persian options
	PersianOpts PersianOptions `json:"persian_opts"`

	// Font
	FontPath string `json:"font_path,omitempty"`

	// File tracking
	ExtractedFiles []string `json:"extracted_files,omitempty"`
	ModifiedFiles  []string `json:"modified_files,omitempty"`
	// ModifiedFileHashes bind staged output to the exact installed source files
	// used during the last successful injection.
	ModifiedFileHashes map[string]string `json:"modified_file_hashes,omitempty"`

	// Internal (not serialized)
	projectFile string
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

// ToolRegistry is the interface for accessing external tools.
// This allows mocking in tests.
type ToolRegistry interface {
	IsAvailable(name string) bool
	GetPath(name string) string
	GetPython() string
	Summary() string
}

// ─── Pipeline ──────────────────────────────────────────────────────

// PipelineStage represents a stage in the localization pipeline.
type PipelineStage string

const (
	StageScan         PipelineStage = "scan"
	StageDetect       PipelineStage = "detect"
	StageDiscover     PipelineStage = "discover"
	StageExtract      PipelineStage = "extract"
	StageTranslate    PipelineStage = "translate"
	StageValidate     PipelineStage = "validate"
	StageInject       PipelineStage = "inject"
	StageVerify       PipelineStage = "verify"
	StagePatchBuild   PipelineStage = "patch_build"
	StageInstallBuild PipelineStage = "install_build"
)

// PipelineStatus tracks the current state of a pipeline execution.
type PipelineStatus struct {
	Stage    PipelineStage `json:"stage"`
	Progress float64       `json:"progress"`
	Message  string        `json:"message"`
	Error    string        `json:"error,omitempty"`
}

// ─── Patch / Installer ─────────────────────────────────────────────

// PatchManifest describes a Persian localization patch.
type PatchManifest struct {
	PatchName    string      `json:"patch_name"`
	GameName     string      `json:"game_name"`
	GameExe      string      `json:"game_exe"`
	Engine       string      `json:"engine"`
	EngineVer    string      `json:"engine_version,omitempty"`
	GameVersion  string      `json:"game_version,omitempty"`
	PatchVersion string      `json:"patch_version"`
	Language     string      `json:"language"`
	CreatedAt    time.Time   `json:"created_at"`
	CreatedBy    string      `json:"created_by"`
	Files        []PatchFile `json:"files"`
	FontFile     string      `json:"font_file,omitempty"`
	Description  string      `json:"description,omitempty"`
}

// PatchFile represents one file in a patch.
type PatchFile struct {
	RelativePath string `json:"relative_path"`
	PatchPath    string `json:"patch_path"`
	OriginalHash string `json:"original_hash"`
	PatchedHash  string `json:"patched_hash"`
	Size         int64  `json:"size"`
	Action       string `json:"action"` // "modify", "add", "replace"
}
