package pcsx2

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PadButtons is the DualShock2 binding order shown in the UI.
var PadButtons = []string{
	"Up", "Down", "Left", "Right",
	"Triangle", "Circle", "Cross", "Square",
	"Select", "Start",
	"L1", "L2", "R1", "R2", "L3", "R3",
	"LUp", "LDown", "LLeft", "LRight",
	"RUp", "RDown", "RLeft", "RRight",
}

func padINIPath(dataDir string) string {
	return filepath.Join(DataRoot(dataDir), "inis", "PCSX2.ini")
}

func validPadButton(button string) bool {
	for _, b := range PadButtons {
		if b == button {
			return true
		}
	}
	return false
}

// sdlBinding matches raw joystick tokens from PCSX2's SDL source
// (SDL-<id>/JoyButton<N>, SDL-<id>/+JoyAxis<N>, SDL-<id>/Hat<N><dir>).
var sdlBinding = regexp.MustCompile(`^SDL-\d+/(JoyButton\d+|[+-]JoyAxis\d+|FullJoyAxis\d+|Hat\d(North|East|South|West))$`)

// ReadPadBindings returns the current [Pad1] bindings. A missing file or
// section yields an empty map, PCSX2 fills defaults on its first launch.
func ReadPadBindings(dataDir string) (map[string]string, error) {
	out := map[string]string{}
	raw, err := os.ReadFile(padINIPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if section != "[Pad1]" {
			continue
		}
		key, val, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if validPadButton(key) {
			out[key] = strings.TrimSpace(val)
		}
	}
	return out, nil
}

// SetPadBinding writes one [Pad1] binding, preserving the rest of the file.
// An empty binding removes the line. It also ensures keyboard input is
// enabled and the pad stays a DualShock2.
func SetPadBinding(dataDir, button, binding string) error {
	if !validPadButton(button) {
		return fmt.Errorf("unknown pad button %q", button)
	}
	if strings.ContainsAny(binding, "\n=[]") {
		return fmt.Errorf("invalid binding %q", binding)
	}
	if binding != "" && !strings.HasPrefix(binding, "Keyboard/") && !sdlBinding.MatchString(binding) {
		return fmt.Errorf("invalid binding %q", binding)
	}
	path := padINIPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []string
	if raw, err := os.ReadFile(path); err == nil {
		lines = strings.Split(string(raw), "\n")
	}
	lines = setINILine(lines, "InputSources", "Keyboard", "true")
	lines = setINILine(lines, "Pad1", "Type", "DualShock2")
	if binding == "" {
		lines = deleteINILine(lines, "Pad1", button)
	} else {
		lines = setINILine(lines, "Pad1", button, binding)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func setINILine(lines []string, section, key, val string) []string {
	header := "[" + section + "]"
	secIdx, keyIdx, endIdx := -1, -1, -1
	cur := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if secIdx >= 0 && endIdx < 0 {
				endIdx = i
			}
			cur = trimmed
			if cur == header && secIdx < 0 {
				secIdx = i
			}
			continue
		}
		if cur == header {
			if k, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(k) == key {
				keyIdx = i
			}
		}
	}
	if keyIdx >= 0 {
		lines[keyIdx] = key + " = " + val
		return lines
	}
	if secIdx < 0 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		return append(lines, header, key+" = "+val)
	}
	if endIdx < 0 {
		endIdx = len(lines)
	}
	lines = append(lines[:endIdx], append([]string{key + " = " + val}, lines[endIdx:]...)...)
	return lines
}

func deleteINILine(lines []string, section, key string) []string {
	header := "[" + section + "]"
	cur := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			cur = trimmed
			continue
		}
		if cur == header {
			if k, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(k) == key {
				return append(lines[:i], lines[i+1:]...)
			}
		}
	}
	return lines
}
