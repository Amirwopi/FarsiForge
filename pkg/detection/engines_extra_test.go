package detection

import (
	"os"
	"path/filepath"
	"testing"
)

// ── resolveRoots tests ──────────────────────────────────────────────

func TestResolveRoots_SkipsRedistDirs(t *testing.T) {
	dir := t.TempDir()

	// Create a redist dir and a real game subdir
	for _, name := range []string{"_CommonRedist", "DirectX", "Game"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	roots := resolveRoots(dir)
	for _, r := range roots {
		base := filepath.Base(r)
		if base == "_CommonRedist" || base == "DirectX" {
			t.Errorf("resolveRoots should skip %q, got root %q", base, r)
		}
	}

	// The input dir itself must always be first
	if len(roots) == 0 || roots[0] != dir {
		t.Fatalf("resolveRoots should return input dir first, got %v", roots)
	}
}

// ── SAGE tests ──────────────────────────────────────────────────────

func TestSAGEDetector_ZeroHour(t *testing.T) {
	dir := t.TempDir()

	dataDir := filepath.Join(dir, "Data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"generals.exe", "EnglishZH.big", "BINKW32.DLL"} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := &SAGEDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected SAGE detection, got nil")
	}
	if res.Engine != "sage" {
		t.Errorf("Engine = %q, want %q", res.Engine, "sage")
	}
	if res.Confidence < 0.85 {
		t.Errorf("Confidence = %.2f, want >= 0.85", res.Confidence)
	}
	if res.Metadata["resolved_root"] != dir {
		t.Errorf("resolved_root = %q, want %q", res.Metadata["resolved_root"], dir)
	}
}

func TestSAGEDetector_RA3(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "RA3.exe"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ra3_english_1.0.SkuDef"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, "Data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "Core5.big"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &SAGEDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected SAGE detection, got nil")
	}
	if res.Engine != "sage" {
		t.Errorf("Engine = %q, want %q", res.Engine, "sage")
	}
}

func TestSAGEDetector_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	d := &SAGEDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil for empty dir, got engine=%q", res.Engine)
	}
}

// ── GoldSrc tests ────────────────────────────────────────────────────

func TestGoldSrcDetector(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"hl.exe", "hw.dll", "FileSystem_Stdio.dll"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "valve"), 0755); err != nil {
		t.Fatal(err)
	}

	d := &GoldSrcDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected GoldSrc detection, got nil")
	}
	if res.Engine != "goldsrc" {
		t.Errorf("Engine = %q, want %q", res.Engine, "goldsrc")
	}
	if res.Confidence < 0.85 {
		t.Errorf("Confidence = %.2f, want >= 0.85", res.Confidence)
	}
}

func TestGoldSrcDetector_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	d := &GoldSrcDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil for empty dir, got engine=%q", res.Engine)
	}
}

// ── FromSoftware tests ──────────────────────────────────────────────

func TestFromSoftwareDetector(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"eldenring.exe", "Data0.bdt", "Data0.bhd", "regulation.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := &FromSoftwareDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected FromSoftware detection, got nil")
	}
	if res.Engine != "fromsoftware" {
		t.Errorf("Engine = %q, want %q", res.Engine, "fromsoftware")
	}
	if res.Confidence < 0.85 {
		t.Errorf("Confidence = %.2f, want >= 0.85", res.Confidence)
	}
}

func TestFromSoftwareDetector_NestedRoot(t *testing.T) {
	parent := t.TempDir()
	gameDir := filepath.Join(parent, "Game")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"eldenring.exe", "Data0.bdt", "Data0.bhd", "regulation.bin"} {
		if err := os.WriteFile(filepath.Join(gameDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := &FromSoftwareDetector{}
	res, err := d.Detect(parent)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected FromSoftware detection from nested root, got nil")
	}
	if res.Metadata["resolved_root"] != gameDir {
		t.Errorf("resolved_root = %q, want %q", res.Metadata["resolved_root"], gameDir)
	}
}

// ── Factorio tests ──────────────────────────────────────────────────

func TestFactorioDetector_NestedRoot(t *testing.T) {
	parent := t.TempDir()
	gameDir := filepath.Join(parent, "Factorio")
	if err := os.MkdirAll(filepath.Join(gameDir, "bin", "x64"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gameDir, "data", "core"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "config-path.cfg"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gameDir, "bin", "x64", "factorio.exe"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &FactorioDetector{}
	res, err := d.Detect(parent)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected Factorio detection from nested root, got nil")
	}
	if res.Engine != "factorio" {
		t.Errorf("Engine = %q, want %q", res.Engine, "factorio")
	}
	if res.Metadata["resolved_root"] != gameDir {
		t.Errorf("resolved_root = %q, want %q", res.Metadata["resolved_root"], gameDir)
	}
}

func TestFactorioDetector_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	d := &FactorioDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil for empty dir, got engine=%q", res.Engine)
	}
}

// ── Zomboid tests ───────────────────────────────────────────────────

func TestZomboidDetector(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "ProjectZomboid64.exe"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zombie", "media", "jre64"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	d := &ZomboidDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected Zomboid detection, got nil")
	}
	if res.Engine != "zomboid" {
		t.Errorf("Engine = %q, want %q", res.Engine, "zomboid")
	}
	if res.Confidence < 0.85 {
		t.Errorf("Confidence = %.2f, want >= 0.85", res.Confidence)
	}
}

func TestZomboidDetector_NoMarkers(t *testing.T) {
	dir := t.TempDir()

	d := &ZomboidDetector{}
	res, err := d.Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil for empty dir, got engine=%q", res.Engine)
	}
}
