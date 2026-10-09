package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestInfoAcceptsStructuredFields(t *testing.T) {
	var output bytes.Buffer
	logger := New("test")
	logger.AddWriter(&output)
	logger.Info("extraction completed", "engine", "unity", "entries", 12)

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	if entries[0].Fields["engine"] != "unity" || entries[0].Fields["entries"] != 12 {
		t.Fatalf("structured fields = %#v", entries[0].Fields)
	}
	if !strings.Contains(output.String(), "engine=unity") || !strings.Contains(output.String(), "entries=12") {
		t.Fatalf("formatted log line omitted fields: %q", output.String())
	}
	if strings.Contains(output.String(), "%!(EXTRA") {
		t.Fatalf("structured fields were treated as printf arguments: %q", output.String())
	}
}

func TestInfoRetainsPrintfFormatting(t *testing.T) {
	var output bytes.Buffer
	logger := New("test")
	logger.AddWriter(&output)
	logger.Info("processed %d entries", 3)
	if !strings.Contains(output.String(), "processed 3 entries") {
		t.Fatalf("printf log message = %q", output.String())
	}
}
