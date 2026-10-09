package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupAndRestoreRejectPathsOutsideGameRoot(t *testing.T) {
	gameRoot := t.TempDir()
	manager := New(gameRoot, filepath.Join(t.TempDir(), "backup"))

	if _, _, err := manager.BackupFile("../outside.bin"); err == nil {
		t.Fatal("BackupFile accepted a path outside the game root")
	}
	if err := manager.RestoreFile(`..\outside.bin`, ""); err == nil {
		t.Fatal("RestoreFile accepted a path outside the game root")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(gameRoot), "outside.bin")); !os.IsNotExist(err) {
		t.Fatalf("unexpected outside file: %v", err)
	}
}
