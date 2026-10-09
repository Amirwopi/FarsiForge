package installer

import (
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
