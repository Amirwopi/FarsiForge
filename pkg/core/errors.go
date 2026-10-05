package core

import "fmt"

// FarsiForgeError is the structured error type used throughout the project.
// It provides context about what operation failed, why, and possible fixes.
type FarsiForgeError struct {
	Op       string   // Operation that failed (e.g., "detect", "extract")
	Engine   string   // Engine involved, if any
	File     string   // File involved, if any
	Err      error    // Underlying error
	Message  string   // Human-readable message
	CanRetry bool     // Whether retrying might help
	Fixes    []string // Possible solutions
}

func (e *FarsiForgeError) Error() string {
	s := fmt.Sprintf("[%s]", e.Op)
	if e.Engine != "" {
		s += fmt.Sprintf(" engine=%s", e.Engine)
	}
	if e.File != "" {
		s += fmt.Sprintf(" file=%s", e.File)
	}
	s += ": " + e.Message
	if e.Err != nil {
		s += fmt.Sprintf(" (%v)", e.Err)
	}
	return s
}

func (e *FarsiForgeError) Unwrap() error {
	return e.Err
}

// NewError creates a new FarsiForgeError.
func NewError(op, message string) *FarsiForgeError {
	return &FarsiForgeError{
		Op:      op,
		Message: message,
	}
}

// Wrap wraps an existing error with FarsiForge context.
func Wrap(op string, err error, message string) *FarsiForgeError {
	return &FarsiForgeError{
		Op:      op,
		Err:     err,
		Message: message,
	}
}

// WrapFile wraps an error with file context.
func WrapFile(op, file string, err error, message string) *FarsiForgeError {
	return &FarsiForgeError{
		Op:      op,
		File:    file,
		Err:     err,
		Message: message,
	}
}

// ErrEngineNotDetected is returned when no engine detector matches.
var ErrEngineNotDetected = NewError("detect", "no known engine detected")

// ErrToolNotFound is returned when a required external tool is missing.
func ErrToolNotFound(toolName string) *FarsiForgeError {
	return &FarsiForgeError{
		Op:      "tools",
		Message: fmt.Sprintf("required tool not available: %s", toolName),
		Fixes: []string{
			fmt.Sprintf("Install or extract %s in the Tools directory", toolName),
			"Check that the tool's executable exists and is not corrupted",
		},
	}
}

// ErrExtractionFailed is returned when text extraction fails.
func ErrExtractionFailed(engine string, err error) *FarsiForgeError {
	return &FarsiForgeError{
		Op:       "extract",
		Engine:   engine,
		Err:      err,
		Message:  "text extraction failed",
		CanRetry: true,
	}
}

// ErrInjectionFailed is returned when translation injection fails.
func ErrInjectionFailed(engine string, err error) *FarsiForgeError {
	return &FarsiForgeError{
		Op:       "inject",
		Engine:   engine,
		Err:      err,
		Message:  "translation injection failed",
		CanRetry: true,
		Fixes: []string{
			"Ensure the game files are not read-only",
			"Ensure no other program has the game files open",
			"Try running FarsiForge as administrator",
		},
	}
}
