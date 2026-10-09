//go:build windows

package core

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSavePreservesExistingProjectWhenReplacementIsBlocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.json")
	project := NewProject("before", t.TempDir(), "unity")
	if err := project.Save(path); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("lock destination against replacement: %v", err)
	}

	project.Name = "after"
	if err := project.Save(path); err == nil {
		_ = windows.CloseHandle(lock)
		t.Fatal("Save succeeded while the destination denied delete sharing")
	}
	if err := windows.CloseHandle(lock); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, previous) {
		t.Fatal("failed replacement changed the previous project file")
	}
	temporary, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".project-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 {
		t.Fatalf("failed replacement left temporary project files: %v", temporary)
	}
}
