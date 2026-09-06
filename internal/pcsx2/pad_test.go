package pcsx2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPadBindingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ini := filepath.Join(DataRoot(dir), "inis", "PCSX2.ini")
	if err := os.MkdirAll(filepath.Dir(ini), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := "[UI]\nSetupWizardIncomplete = false\n\n[Pad1]\nType = DualShock2\nCross = Keyboard/K\n"
	if err := os.WriteFile(ini, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPadBindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["Cross"] != "Keyboard/K" {
		t.Errorf("cross = %q", got["Cross"])
	}
	if err := SetPadBinding(dir, "Circle", "Keyboard/L"); err != nil {
		t.Fatal(err)
	}
	if err := SetPadBinding(dir, "Cross", "Keyboard/X"); err != nil {
		t.Fatal(err)
	}
	got, err = ReadPadBindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["Circle"] != "Keyboard/L" || got["Cross"] != "Keyboard/X" {
		t.Errorf("bindings = %v", got)
	}
	raw, _ := os.ReadFile(ini)
	s := string(raw)
	for _, want := range []string{"SetupWizardIncomplete = false", "Type = DualShock2", "[InputSources]", "Keyboard = true"} {
		if !strings.Contains(s, want) {
			t.Errorf("must preserve %q, got:\n%s", want, s)
		}
	}
	if err := SetPadBinding(dir, "Cross", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadPadBindings(dir)
	if _, ok := got["Cross"]; ok {
		t.Error("empty binding must remove the line")
	}
	if err := SetPadBinding(dir, "Nope", "Keyboard/A"); err == nil {
		t.Error("unknown button must fail")
	}
	if err := SetPadBinding(dir, "Cross", "SDL-0/JoyButton1"); err != nil {
		t.Errorf("raw joystick token must pass: %v", err)
	}
	if err := SetPadBinding(dir, "LUp", "SDL-0/-JoyAxis1"); err != nil {
		t.Errorf("raw axis token must pass: %v", err)
	}
	if err := SetPadBinding(dir, "Cross", "SDL-0/FaceSouth"); err == nil {
		t.Error("semantic gamecontroller names need the SDL mapping DB, raw tokens only")
	}
}

func TestPadBindingFreshFile(t *testing.T) {
	dir := t.TempDir()
	if err := SetPadBinding(dir, "Start", "Keyboard/Return"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPadBindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["Start"] != "Keyboard/Return" || got["Type"] != "" {
		t.Errorf("bindings = %v (Type is not a button, must be skipped)", got)
	}
}
