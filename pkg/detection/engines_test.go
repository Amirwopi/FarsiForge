package detection

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ── PCK version byte mapping tests ──────────────────────────────────

// writePCKFixture creates a minimal synthetic .pck file with the given
// magic and pack version. Returns the file path.
func writePCKFixture(t *testing.T, dir, name, magic string, packVersion uint32) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create pck fixture: %v", err)
	}
	defer f.Close()

	// Write 4-byte magic + 4-byte version (LE) + padding
	if _, err := f.WriteString(magic); err != nil {
		t.Fatalf("write magic: %v", err)
	}
	var verBytes [4]byte
	binary.LittleEndian.PutUint32(verBytes[:], packVersion)
	if _, err := f.Write(verBytes[:]); err != nil {
		t.Fatalf("write version: %v", err)
	}
	// Pad to 16 bytes so the file isn't suspiciously tiny
	pad := make([]byte, 8)
	if _, err := f.Write(pad); err != nil {
		t.Fatalf("write padding: %v", err)
	}
	return path
}

func TestPCKVersionToString(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		want    string
	}{
		{"v1 maps to Godot 2.x", 1, "2.x"},
		{"v2 maps to Godot 3.x / 4.0-4.2", 2, "3.x / 4.0-4.2"},
		{"v3 maps to Godot 4.3+", 3, "4.3+"},
		{"unknown version", 99, "pack_ver=99"},
		{"version 0", 0, "pack_ver=0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pckVersionToString(tt.version)
			if got != tt.want {
				t.Errorf("pckVersionToString(%d) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestReadPCKHeader(t *testing.T) {
	dir := t.TempDir()

	// Create a valid v3 pck fixture
	pckPath := writePCKFixture(t, dir, "game.pck", "GDPC", 3)

	magic, version, err := readPCKHeader(pckPath)
	if err != nil {
		t.Fatalf("readPCKHeader failed: %v", err)
	}
	if magic != "GDPC" {
		t.Errorf("magic = %q, want %q", magic, "GDPC")
	}
	if version != 3 {
		t.Errorf("version = %d, want 3", version)
	}
}

func TestReadPCKHeader_NonexistentFile(t *testing.T) {
	_, _, err := readPCKHeader(filepath.Join(t.TempDir(), "nonexistent.pck"))
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadPCKHeader_BadMagic(t *testing.T) {
	dir := t.TempDir()
	pckPath := writePCKFixture(t, dir, "badmagic.pck", "XXXX", 3)

	magic, _, err := readPCKHeader(pckPath)
	if err != nil {
		t.Fatalf("readPCKHeader failed: %v", err)
	}
	if magic == "GDPC" {
		t.Error("expected non-GDPC magic, got GDPC")
	}
}

func TestReadPCKVersion(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		magic   string
		version uint32
		want    string
	}{
		{"GDPC v1", "GDPC", 1, "2.x"},
		{"GDPC v2", "GDPC", 2, "3.x / 4.0-4.2"},
		{"GDPC v3", "GDPC", 3, "4.3+"},
		{"bad magic", "XXXX", 3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writePCKFixture(t, dir, tt.name+".pck", tt.magic, tt.version)
			got := readPCKVersion(path)
			if got != tt.want {
				t.Errorf("readPCKVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── UE3 marker tests ────────────────────────────────────────────────

func TestUE3Detector_CoalescedMarkers(t *testing.T) {
	dir := t.TempDir()

	// Create Coalesced.INT and Coalesced.ENG files in a Localization subdir
	locDir := filepath.Join(dir, "Localization")
	if err := os.MkdirAll(locDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Coalesced.INT", "Coalesced.ENG"} {
		if err := os.WriteFile(filepath.Join(locDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := &UE3Detector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected UE3 detection, got nil")
	}
	if res.Engine != "ue3" {
		t.Errorf("Engine = %q, want %q", res.Engine, "ue3")
	}
	if res.Version != "UE3" {
		t.Errorf("Version = %q, want %q", res.Version, "UE3")
	}
}

func TestUE3Detector_XXXExtension(t *testing.T) {
	dir := t.TempDir()

	// Create .xxx files (MK-style)
	assetDir := filepath.Join(dir, "Asset")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Map1.xxx", "Char.xxx"} {
		if err := os.WriteFile(filepath.Join(assetDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := &UE3Detector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected UE3 detection from .xxx files, got nil")
	}
	if res.Engine != "ue3" {
		t.Errorf("Engine = %q, want %q", res.Engine, "ue3")
	}
}

func TestUE3Detector_UPKFiles(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "level1.upk"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "level2.upk"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &UE3Detector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected UE3 detection from .upk files, got nil")
	}
	if res.Engine != "ue3" {
		t.Errorf("Engine = %q, want %q", res.Engine, "ue3")
	}
}

func TestUE3Detector_NoFalsePositiveOnUE4(t *testing.T) {
	dir := t.TempDir()

	// Create UE4/UE5 markers: .pak files, Content dir, NO UE3 markers
	contentDir := filepath.Join(dir, "Content", "Paks")
	if err := os.MkdirAll(contentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contentDir, "game-WindowsClient.pak"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &UE3Detector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("UE3 detector should NOT match UE4/UE5 games, got engine=%q", res.Engine)
	}
}

func TestUE3Detector_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	// Empty directory — no markers at all
	d := &UE3Detector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil for empty dir, got engine=%q", res.Engine)
	}
}

// ── Launcher guard tests ────────────────────────────────────────────

func TestIsLauncherExe(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		want bool
	}{
		{"FortniteLauncher.exe", "FortniteLauncher.exe", true},
		{"launcher.exe", "launcher.exe", true},
		{"GameLauncher.exe", "GameLauncher.exe", true},
		{"LAUNCHER.exe", "LAUNCHER.exe", true},
		{"MyLauncher.exe", "MyLauncher.exe", true},
		{"Game.exe", "Game.exe", false},
		{"Aska.exe", "Aska.exe", false},
		{"FortniteClient-Win64-Shipping.exe", "FortniteClient-Win64-Shipping.exe", false},
		{"UnityCrashHandler64.exe", "UnityCrashHandler64.exe", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLauncherExe(tt.exe)
			if got != tt.want {
				t.Errorf("isLauncherExe(%q) = %v, want %v", tt.exe, got, tt.want)
			}
		})
	}
}

func TestFindGameExeSkippingLaunchers(t *testing.T) {
	dir := t.TempDir()

	// Create a launcher exe and a real game exe
	if err := os.WriteFile(filepath.Join(dir, "GameLauncher.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyGame.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	exePath, gameName := findGameExeSkippingLaunchers(dir, []string{".exe"})
	if exePath == "" {
		t.Fatal("expected to find a game exe, got empty path")
	}
	if isLauncherExe(filepath.Base(exePath)) {
		t.Errorf("should not return a launcher exe, got %q", exePath)
	}
	if gameName != "MyGame" {
		t.Errorf("gameName = %q, want %q", gameName, "MyGame")
	}
}

func TestFindGameExeSkippingLaunchers_OnlyLauncher(t *testing.T) {
	dir := t.TempDir()

	// Only a launcher exists — should fall back to returning it
	if err := os.WriteFile(filepath.Join(dir, "OnlyLauncher.exe"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	exePath, _ := findGameExeSkippingLaunchers(dir, []string{".exe"})
	if exePath == "" {
		t.Fatal("expected fallback to return the launcher exe, got empty path")
	}
}

func TestFindGameExeSkippingLaunchers_NoExe(t *testing.T) {
	dir := t.TempDir()

	exePath, gameName := findGameExeSkippingLaunchers(dir, []string{".exe"})
	if exePath != "" {
		t.Errorf("expected empty path for dir with no exes, got %q", exePath)
	}
	if gameName != "" {
		t.Errorf("expected empty gameName, got %q", gameName)
	}
}
