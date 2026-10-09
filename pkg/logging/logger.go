// Package logging provides structured logging for FarsiForge.
//
// All log entries include timestamp, level, module, and optional
// contextual fields (game, engine, file, operation).
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a log level string.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Fields holds contextual key-value pairs for a log entry.
type Fields map[string]interface{}

// Logger is a structured logger for FarsiForge.
type Logger struct {
	mu      sync.Mutex
	level   Level
	module  string
	writers []io.Writer
	fields  Fields
	entries []Entry // In-memory log buffer for UI
	maxBuf  int
}

// Entry represents a single log entry.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Module    string    `json:"module"`
	Message   string    `json:"message"`
	Fields    Fields    `json:"fields,omitempty"`
}

// New creates a new logger with the given module name.
func New(module string) *Logger {
	return &Logger{
		level:   LevelInfo,
		module:  module,
		writers: []io.Writer{os.Stdout},
		fields:  make(Fields),
		maxBuf:  1000,
	}
}

// SetLevel sets the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// AddWriter adds an additional output writer (e.g., file).
func (l *Logger) AddWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writers = append(l.writers, w)
}

// AddFileWriter adds a log file writer.
func (l *Logger) AddFileWriter(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	l.AddWriter(f)
	return nil
}

// WithFields creates a child logger with additional fields.
func (l *Logger) WithFields(fields Fields) *Logger {
	child := &Logger{
		level:   l.level,
		module:  l.module,
		writers: l.writers,
		fields:  make(Fields),
		entries: l.entries,
		maxBuf:  l.maxBuf,
	}
	for k, v := range l.fields {
		child.fields[k] = v
	}
	for k, v := range fields {
		child.fields[k] = v
	}
	return child
}

// WithModule creates a child logger with a different module name.
func (l *Logger) WithModule(module string) *Logger {
	child := l.WithFields(nil)
	child.module = module
	return child
}

// GetEntries returns the buffered log entries.
func (l *Logger) GetEntries() []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Entry, len(l.entries))
	copy(result, l.entries)
	return result
}

// log writes a log entry at the given level.
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	now := time.Now()
	entry := Entry{
		Timestamp: now,
		Level:     level.String(),
		Module:    l.module,
		Message:   msg,
		Fields:    l.fields,
	}

	// Buffer for UI
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxBuf {
		l.entries = l.entries[len(l.entries)-l.maxBuf:]
	}

	// Format: [2026-10-05 15:30:45] [INFO] [detection] Engine detected: Unity
	line := fmt.Sprintf("[%s] [%-5s] [%s] %s",
		now.Format("2006-01-02 15:04:05"),
		level.String(),
		l.module,
		msg,
	)

	// Add fields
	if len(l.fields) > 0 {
		parts := make([]string, 0, len(l.fields))
		for k, v := range l.fields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		line += " {" + strings.Join(parts, ", ") + "}"
	}

	line += "\n"

	for _, w := range l.writers {
		// Logging is best-effort and must not break the operation being logged.
		_, _ = w.Write([]byte(line))
	}

	if level == LevelFatal {
		os.Exit(1)
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...interface{}) { l.log(LevelDebug, msg, args...) }

// Info logs an info message.
func (l *Logger) Info(msg string, args ...interface{}) { l.log(LevelInfo, msg, args...) }

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...interface{}) { l.log(LevelWarn, msg, args...) }

// Error logs an error message.
func (l *Logger) Error(msg string, args ...interface{}) { l.log(LevelError, msg, args...) }

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string, args ...interface{}) { l.log(LevelFatal, msg, args...) }

// defaultLogger is the package-level default logger.
var defaultLogger = New("farsiforge")

// Default returns the default logger.
func Default() *Logger { return defaultLogger }

// SetDefault replaces the default logger.
func SetDefault(l *Logger) { defaultLogger = l }
