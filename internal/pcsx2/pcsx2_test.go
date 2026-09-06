package pcsx2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppImageURL(t *testing.T) {
	u := AppImageURL("v2.9.30")
	if !strings.Contains(u, "PCSX2/pcsx2/releases/download/v2.9.30/pcsx2-v2.9.30-linux-appimage-x64-Qt.AppImage") {
		t.Errorf("url = %q", u)
	}
}

func TestAppImagePath(t *testing.T) {
	p := AppImagePath("/d", "v2.9.30")
	if !strings.HasSuffix(p, "pcsx2-v2.9.30.AppImage") {
		t.Errorf("path = %q", p)
	}
}

func TestInstallBIOS(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	srcDir := t.TempDir()
	a := srcDir + "/a.bin"
	b := srcDir + "/b.rom"
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("bb"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := InstallBIOS(dataDir, []string{a, b, srcDir + "/missing.bin", srcDir})
	if err != nil || n != 2 {
		t.Fatalf("InstallBIOS = (%d, %v)", n, err)
	}
	for _, tc := range []struct{ name, want string }{{"a.bin", "a"}, {"b.rom", "bb"}} {
		got, err := os.ReadFile(BiosDir(dataDir) + "/" + tc.name)
		if err != nil || string(got) != tc.want {
			t.Errorf("%s = %q, %v", tc.name, got, err)
		}
	}
	if HasBIOS(dataDir) != true {
		t.Error("HasBIOS must be true after install")
	}
}

func TestMigrateLegacyBIOS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := t.TempDir()
	legacy := LegacyBiosDir()
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy+"/old.bin", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := MigrateLegacyBIOS(dataDir)
	if err != nil || n != 1 {
		t.Fatalf("migrate = (%d, %v)", n, err)
	}
	if !HasBIOS(dataDir) {
		t.Error("migrated BIOS must be detected")
	}
	if n2, _ := MigrateLegacyBIOS(dataDir); n2 != 0 {
		t.Errorf("second migrate = %d, want 0 (already there)", n2)
	}
}

func TestMigrateStaleDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	stale := filepath.Join(dataDir, "pcsx2", "bios")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale+"/s.bin", []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := MigrateLegacyBIOS(dataDir)
	if err != nil || n != 1 {
		t.Fatalf("migrate = (%d, %v)", n, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "PCSX2", "bios", "s.bin")); err != nil {
		t.Errorf("stale bios not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "pcsx2")); !os.IsNotExist(err) {
		t.Error("stale dir must be removed after migration")
	}
}

func TestPrepareSeedsINI(t *testing.T) {
	dataDir := t.TempDir()
	if err := Prepare(dataDir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(DataRoot(dataDir) + "/inis/PCSX2.ini")
	if err != nil {
		t.Fatal(err)
	}
	ini := string(raw)
	if !strings.Contains(ini, "SetupWizardIncomplete = false") {
		t.Errorf("wizard flag missing:\n%s", ini)
	}
	if !strings.Contains(ini, "Bios = "+BiosDir(dataDir)) {
		t.Errorf("bios path missing:\n%s", ini)
	}
	if !strings.Contains(ini, "SettingsVersion = 1") {
		t.Error("seed must carry SettingsVersion or PCSX2 resets it")
	}
	if !strings.Contains(ini, "[EmuCore]") {
		t.Error("seed must be a full genuine ini, not a minimal stub")
	}

	if err := Prepare(dataDir); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(DataRoot(dataDir) + "/inis/PCSX2.ini")
	if string(raw2) != ini {
		t.Error("second Prepare must not change the ini")
	}
}

func TestPreparePatchesExistingINI(t *testing.T) {
	dataDir := t.TempDir()
	if err := Prepare(dataDir); err != nil {
		t.Fatal(err)
	}
	iniPath := DataRoot(dataDir) + "/inis/PCSX2.ini"
	raw, _ := os.ReadFile(iniPath)
	patched := strings.Replace(string(raw), "SetupWizardIncomplete = false", "SetupWizardIncomplete = true", 1)
	if err := os.WriteFile(iniPath, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(dataDir); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(iniPath)
	if !strings.Contains(string(raw2), "SetupWizardIncomplete = false") {
		t.Error("Prepare must flip the wizard flag back to false")
	}
}

func TestLaunchArgs(t *testing.T) {
	args := LaunchArgs("/data", "/g/game.iso")
	want := []string{"-batch", "-fastboot", "-fullscreen", "-datapath", "/data", "/g/game.iso"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v", args)
	}
}

func TestDataRootNesting(t *testing.T) {
	if DataRoot("/data") != "/data/PCSX2" {
		t.Errorf("DataRoot = %q", DataRoot("/data"))
	}
}
