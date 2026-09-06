package save

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCardPaths(t *testing.T) {
	dir := t.TempDir()
	if got := LocalCard(dir, "G", 1); got != filepath.Join(dir, "cards", "G", "slot1.ps2") {
		t.Errorf("local = %q", got)
	}
	if got := LocalCard(dir, "G", 2); got != filepath.Join(dir, "cards", "G", "slot2.ps2") {
		t.Errorf("local2 = %q", got)
	}
	if got := WorkingCard(dir, 1); got != filepath.Join(dir, "PCSX2", "memcards", "Mcd001.ps2") {
		t.Errorf("working default = %q", got)
	}
}

func TestSlotFilenamesFromINI(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "PCSX2", "inis"), 0o755); err != nil {
		t.Fatal(err)
	}
	ini := "[MemoryCards]\nSlot1_Filename = Custom1.ps2\nSlot2_Filename = Custom2.ps2\n"
	if err := os.WriteFile(filepath.Join(dir, "PCSX2", "inis", "PCSX2.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := WorkingCard(dir, 1); got != filepath.Join(dir, "PCSX2", "memcards", "Custom1.ps2") {
		t.Errorf("working custom = %q", got)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st := LoadState(dir)
	st = st.Set("G", 1, Synced{SHA: "abc"})
	if err := SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	loaded := LoadState(dir)
	s, ok := loaded.Get("G", 1)
	if !ok || s.SHA != "abc" {
		t.Errorf("state = %+v", loaded)
	}
	if _, ok := loaded.Get("G", 2); ok {
		t.Error("slot 2 must be absent")
	}
}

func TestBackupLocalRotation(t *testing.T) {
	dir := t.TempDir()
	lib := LocalCard(dir, "G", 1)
	if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(lib, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := BackupLocal(dir, "G", 1); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(lib))
	if err != nil {
		t.Fatal(err)
	}
	baks := 0
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".bak" {
			baks++
		}
	}
	if baks != 3 {
		t.Errorf("backups = %d, want 3", baks)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "sub", "b.bin")
	if err := CopyFile(dst, src); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil || string(raw) != "data" {
		t.Errorf("copy = %q, %v", raw, err)
	}
}
