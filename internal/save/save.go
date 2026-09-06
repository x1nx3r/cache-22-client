// Package save manages per-game memory cards: a local cards library,
// working copies in the PCSX2 memcards dir, and last-synced hashes for
// three-way sync against the server.
package save

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Synced struct {
	SHA   string `json:"sha"`
	MTime int64  `json:"mtime"`
}

type State struct {
	Games map[string]map[int]Synced `json:"games"`
}

func CardsDir(dataDir string) string {
	return filepath.Join(dataDir, "cards")
}

// LocalCard is the library copy for (serial, slot).
func LocalCard(dataDir, serial string, slot int) string {
	return filepath.Join(CardsDir(dataDir), serial, slotName(slot))
}

func slotName(slot int) string {
	if slot == 2 {
		return "slot2.ps2"
	}
	return "slot1.ps2"
}

// slotFilenames reads Slot1_Filename/Slot2_Filename from the PCSX2 ini,
// falling back to the stock Mcd001/Mcd002 names.
func slotFilenames(dataDir string) (string, string) {
	s1, s2 := "Mcd001.ps2", "Mcd002.ps2"
	raw, err := os.ReadFile(filepath.Join(dataDir, "PCSX2", "inis", "PCSX2.ini"))
	if err != nil {
		return s1, s2
	}
	section := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if section != "[MemoryCards]" {
			continue
		}
		if k, v, ok := strings.Cut(trimmed, "="); ok {
			switch strings.TrimSpace(k) {
			case "Slot1_Filename":
				if name := filepath.Base(strings.TrimSpace(v)); name != "" {
					s1 = name
				}
			case "Slot2_Filename":
				if name := filepath.Base(strings.TrimSpace(v)); name != "" {
					s2 = name
				}
			}
		}
	}
	return s1, s2
}

// WorkingCard is the live file PCSX2 reads/writes for (serial, slot).
// Slot files live directly in the shared memcards dir.
func WorkingCard(dataDir string, slot int) string {
	s1, s2 := slotFilenames(dataDir)
	name := s1
	if slot == 2 {
		name = s2
	}
	return filepath.Join(dataDir, "PCSX2", "memcards", name)
}

func HashFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func statePath(dataDir string) string {
	return filepath.Join(dataDir, "save_state.json")
}

func LoadState(dataDir string) State {
	var st State
	raw, err := os.ReadFile(statePath(dataDir))
	if err != nil {
		return State{Games: map[string]map[int]Synced{}}
	}
	if err := json.Unmarshal(raw, &st); err != nil || st.Games == nil {
		return State{Games: map[string]map[int]Synced{}}
	}
	return st
}

func (st State) Get(serial string, slot int) (Synced, bool) {
	slots, ok := st.Games[serial]
	if !ok {
		return Synced{}, false
	}
	s, ok := slots[slot]
	return s, ok
}

func (st State) Set(serial string, slot int, s Synced) State {
	if st.Games == nil {
		st.Games = map[string]map[int]Synced{}
	}
	slots := st.Games[serial]
	if slots == nil {
		slots = map[int]Synced{}
	}
	slots[slot] = s
	st.Games[serial] = slots
	return st
}

func SaveState(dataDir string, st State) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(dataDir), raw, 0o644)
}

// BackupLocal copies the library card aside, keeping the newest 3.
func BackupLocal(dataDir, serial string, slot int) error {
	src := LocalCard(dataDir, serial, slot)
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	dst := filepath.Join(dir, fmt.Sprintf("%s.local.%d.bak", base, time.Now().UnixNano()))
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	prefix := base + ".local."
	var baks []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			baks = append(baks, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(baks)
	for len(baks) > 3 {
		os.Remove(baks[0])
		baks = baks[1:]
	}
	return nil
}

func CopyFile(dst, src string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
