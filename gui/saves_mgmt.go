package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/x1nx3r/cache-22-client/internal/api"

	"github.com/x1nx3r/cache-22-client/internal/save"
)

type SaveBackupInfo struct {
	Name  string `json:"name"`
	MTime int64  `json:"mtime"`
	Size  int64  `json:"size"`
}

type SaveSlotInfo struct {
	Slot    int              `json:"slot"`
	Present bool             `json:"present"`
	Size    int64            `json:"size"`
	MTime   int64            `json:"mtime"`
	Synced  bool             `json:"synced"`
	Backups []SaveBackupInfo `json:"backups"`
}

type GameSaveInfo struct {
	Serial string         `json:"serial"`
	Slots  []SaveSlotInfo `json:"slots"`
}

func (e *EmulatorService) requireStopped() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.proc != nil {
		return errors.New("stop the emulator first")
	}
	return nil
}

// ListSaves scans the local cards library, newest serials first.
func (e *EmulatorService) ListSaves() ([]GameSaveInfo, error) {
	entries, err := os.ReadDir(save.CardsDir(e.dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return []GameSaveInfo{}, nil
		}
		return nil, err
	}
	st := save.LoadState(e.dataDir)
	var out []GameSaveInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		serial := entry.Name()
		info := GameSaveInfo{Serial: serial}
		for _, slot := range []int{1, 2} {
			si := SaveSlotInfo{Slot: slot, Backups: []SaveBackupInfo{}}
			lib := save.LocalCard(e.dataDir, serial, slot)
			if fi, err := os.Stat(lib); err == nil {
				si.Present = true
				si.Size = fi.Size()
				si.MTime = fi.ModTime().Unix()
				if h, herr := save.HashFile(lib); herr == nil {
					if last, ok := st.Get(serial, slot); ok && last.SHA == h {
						si.Synced = true
					}
				}
			}
			dir := filepath.Dir(lib)
			prefix := filepath.Base(lib) + "."
			if files, err := os.ReadDir(dir); err == nil {
				for _, f := range files {
					name := f.Name()
					if f.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".bak") {
						continue
					}
					fi, err := f.Info()
					if err != nil {
						continue
					}
					si.Backups = append(si.Backups, SaveBackupInfo{Name: name, MTime: fi.ModTime().Unix(), Size: fi.Size()})
				}
				sort.Slice(si.Backups, func(i, j int) bool { return si.Backups[i].MTime > si.Backups[j].MTime })
			}
			info.Slots = append(info.Slots, si)
		}
		keep := false
		for _, sl := range info.Slots {
			if sl.Present || len(sl.Backups) > 0 {
				keep = true
				break
			}
		}
		if keep {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out, nil
}

// RestoreSave copies a local backup over the library and working cards and
// clears sync state so the restored copy wins the next compare.
func (e *EmulatorService) RestoreSave(serial string, slot int, name string) error {
	if err := e.requireStopped(); err != nil {
		return err
	}
	if slot != 1 && slot != 2 {
		return errors.New("bad slot")
	}
	safe := filepath.Base(name)
	if safe != name || !strings.HasSuffix(safe, ".bak") {
		return errors.New("bad backup name")
	}
	lib := save.LocalCard(e.dataDir, serial, slot)
	prefix := filepath.Base(lib) + "."
	if !strings.HasPrefix(safe, prefix) {
		return errors.New("backup does not belong to this slot")
	}
	src := filepath.Join(filepath.Dir(lib), safe)
	if _, err := os.Stat(src); err != nil {
		return errors.New("backup not found")
	}
	if err := save.CopyFile(lib, src); err != nil {
		return err
	}
	if err := save.CopyFile(save.WorkingCard(e.dataDir, slot), src); err != nil {
		return err
	}
	st := save.LoadState(e.dataDir)
	if slots, ok := st.Games[serial]; ok {
		delete(slots, slot)
		if len(slots) == 0 {
			delete(st.Games, serial)
		}
		_ = save.SaveState(e.dataDir, st)
	}
	e.noteSave(true, "Save restored from backup")
	return nil
}

// DeleteSave removes local cards, backups and sync state, plus the cloud copy.
func (e *EmulatorService) DeleteSave(serial string, slot int) error {
	if err := e.requireStopped(); err != nil {
		return err
	}
	if slot != 1 && slot != 2 {
		return errors.New("bad slot")
	}
	lib := save.LocalCard(e.dataDir, serial, slot)
	os.Remove(lib)
	os.Remove(save.WorkingCard(e.dataDir, slot))
	dir := filepath.Dir(lib)
	prefix := filepath.Base(lib) + "."
	if files, err := os.ReadDir(dir); err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasPrefix(f.Name(), prefix) {
				os.Remove(filepath.Join(dir, f.Name()))
			}
		}
	}
	st := save.LoadState(e.dataDir)
	if slots, ok := st.Games[serial]; ok {
		delete(slots, slot)
		if len(slots) == 0 {
			delete(st.Games, serial)
		}
		_ = save.SaveState(e.dataDir, st)
	}
	if active, err := e.servers.ActiveEntry(); err == nil {
		_ = e.deleteRemoteSave(active, serial, slot)
	}
	e.noteSave(true, "Save deleted")
	return nil
}

func (e *EmulatorService) deleteRemoteSave(active ServerEntry, serial string, slot int) error {
	return api.New(active.URL, active.Token).DeleteSave(serial, slot)
}

// SyncSaves pulls newer cloud saves for one game (emulator must be stopped).
func (e *EmulatorService) SyncSaves(serial string) error {
	if err := e.requireStopped(); err != nil {
		return err
	}
	active, err := e.servers.ActiveEntry()
	if err != nil {
		return err
	}
	e.prepareSaves(api.New(active.URL, active.Token), serial)
	return nil
}
