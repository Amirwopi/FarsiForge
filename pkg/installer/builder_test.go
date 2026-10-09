package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanGamePath(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
		bad  bool
	}{
		{name: `Game_Data\resources.assets`, want: "Game_Data/resources.assets"},
		{name: "../outside", bad: true},
		{name: `..\outside`, bad: true},
		{name: `C:\outside`, bad: true},
		{name: "/outside", bad: true},
		{name: `Data/file.bin:stream`, bad: true},
		{name: `CON/settings.txt`, bad: true},
		{name: `Data/trailing.`, bad: true},
		{name: "", bad: true},
	} {
		got, err := cleanGamePath(tc.name)
		if tc.bad {
			if err == nil {
				t.Errorf("cleanGamePath(%q) accepted an unsafe path", tc.name)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("cleanGamePath(%q) = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestValidateBuildConfigRejectsDuplicateCaseInsensitiveTargets(t *testing.T) {
	root := t.TempDir()
	patched := filepath.Join(root, "patched.bin")
	original := filepath.Join(root, "original.bin")
	for _, file := range []string{patched, original} {
		if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := validateBuildConfig(BuildConfig{
		GameRoot: root,
		Targets: []PatchTarget{
			{GamePath: "Data/file.bin", PatchedFile: patched, OriginalFile: original},
			{GamePath: `data\FILE.BIN`, PatchedFile: patched, OriginalFile: original},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate case-insensitive game targets to be rejected")
	}
}

func TestBuildRejectsInvalidInputsBeforeCreatingOutput(t *testing.T) {
	root := t.TempDir()
	patched := filepath.Join(root, "patched.bin")
	if err := os.WriteFile(patched, []byte("translated"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "patch-output")
	_, err := Build(BuildConfig{
		OutputDir: out,
		Targets:   []PatchTarget{{GamePath: "../outside.bin", PatchedFile: patched}},
	})
	if err == nil {
		t.Fatal("Build accepted a target path outside the game directory")
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Fatalf("output directory was created before input validation: %v", statErr)
	}
}

func TestBuildRejectsSourceChangedAfterInjection(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "locale.ini")
	patched := filepath.Join(root, "patched.ini")
	if err := os.WriteFile(original, []byte("title=Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patched, []byte("title=سلام\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256([]byte("title=Hello\n"))
	if err := os.WriteFile(original, []byte("title=Updated by game patch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, "patch")
	_, err := Build(BuildConfig{
		GameRoot:  root,
		OutputDir: output,
		PatchName: "Sample",
		Targets: []PatchTarget{{
			GamePath: "locale.ini", PatchedFile: patched,
			OriginalSHA256: hex.EncodeToString(sourceHash[:]),
		}},
	})
	if err == nil {
		t.Fatal("Build accepted staged output after the game source changed")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("patch output was created before stale source validation: %v", statErr)
	}
}
