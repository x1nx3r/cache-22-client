// Three-way save sync shared by the GUI and the CLI: pull pre-launch,
// push post-exit, last-writer-wins on both-sides-changed with the loser
// backed up locally. Offline-tolerant: every failure path falls back to
// local cards and reports instead of erroring.
package save

import (
	"os"
	"path/filepath"
	"time"

	"github.com/x1nx3r/cache-22-client/internal/api"
)

// Note is one user-facing sync outcome.
type Note struct {
	Msg string
	OK  bool
}

// Report summarizes one Prepare or Push pass over both slots.
type Report struct {
	Offline bool
	Pulled  bool
	Pushed  bool
	Failed  bool
	Notes   []Note
}

// installCard copies the library card into the working slot file.
func installCard(dataDir, serial string, slot int) error {
	return CopyFile(WorkingCard(dataDir, slot), LocalCard(dataDir, serial, slot))
}

// Prepare syncs both slots for a game before launch.
func Prepare(c *api.Client, dataDir, serial string) Report {
	rep := Report{}
	st := LoadState(dataDir)
	dirty := false
	for _, slot := range []int{1, 2} {
		lib := LocalCard(dataDir, serial, slot)
		localHash, lerr := HashFile(lib)
		last, synced := st.Get(serial, slot)
		srvMeta, srvOk, herr := c.HeadSave(serial, slot)
		if herr != nil {
			rep.Offline = true
			if lerr == nil {
				_ = installCard(dataDir, serial, slot)
			}
			continue
		}
		if !srvOk {
			if lerr == nil {
				_ = installCard(dataDir, serial, slot)
				st = st.Set(serial, slot, Synced{SHA: localHash})
				dirty = true
			} else if wh, werr := HashFile(WorkingCard(dataDir, slot)); werr == nil {
				// Adopt the working card as this game's card (first boot).
				_ = CopyFile(lib, WorkingCard(dataDir, slot))
				st = st.Set(serial, slot, Synced{SHA: wh})
				dirty = true
			}
			continue
		}
		if lerr != nil {
			// No local card: take the server's.
			if raw, _, gerr := c.GetSave(serial, slot); gerr == nil {
				_ = os.MkdirAll(filepath.Dir(lib), 0o755)
				_ = os.WriteFile(lib, raw, 0o644)
				_ = installCard(dataDir, serial, slot)
				st = st.Set(serial, slot, Synced{SHA: srvMeta.SHA})
				dirty, rep.Pulled = true, true
			} else {
				rep.Offline = true
			}
			continue
		}
		if !synced {
			if localHash == srvMeta.SHA {
				st = st.Set(serial, slot, Synced{SHA: localHash})
				dirty = true
				_ = installCard(dataDir, serial, slot)
			} else {
				dirty = resolveConflict(c, dataDir, st, serial, slot, localHash, srvMeta, &rep)
				st = LoadState(dataDir)
			}
			continue
		}
		if localHash == last.SHA {
			if srvMeta.SHA != last.SHA {
				if raw, _, gerr := c.GetSave(serial, slot); gerr == nil {
					_ = os.WriteFile(lib, raw, 0o644)
					_ = installCard(dataDir, serial, slot)
					st = st.Set(serial, slot, Synced{SHA: srvMeta.SHA})
					dirty, rep.Pulled = true, true
				} else {
					rep.Offline = true
				}
			} else {
				_ = installCard(dataDir, serial, slot)
			}
			continue
		}
		if srvMeta.SHA == last.SHA {
			_ = installCard(dataDir, serial, slot)
			st = st.Set(serial, slot, Synced{SHA: localHash})
			dirty = true
			continue
		}
		dirty = resolveConflict(c, dataDir, st, serial, slot, localHash, srvMeta, &rep)
		st = LoadState(dataDir)
	}
	if dirty {
		_ = SaveState(dataDir, st)
	}
	switch {
	case rep.Offline:
		rep.Notes = append(rep.Notes, Note{"Saves offline, using local cards", false})
	case rep.Pulled:
		rep.Notes = append(rep.Notes, Note{"Downloaded cloud save", true})
	}
	return rep
}

// resolveConflict settles a both-sides-changed slot by mtime, backing up
// the loser locally. Returns the new dirty value; the caller reloads state
// afterwards.
func resolveConflict(c *api.Client, dataDir string, st State, serial string, slot int, localHash string, srvMeta api.SaveMeta, rep *Report) bool {
	lib := LocalCard(dataDir, serial, slot)
	var localMT time.Time
	if fi, err := os.Stat(lib); err == nil {
		localMT = fi.ModTime()
	}
	if !srvMeta.Updated.IsZero() && srvMeta.Updated.After(localMT) {
		_ = BackupLocal(dataDir, serial, slot)
		if raw, _, gerr := c.GetSave(serial, slot); gerr == nil {
			_ = os.WriteFile(lib, raw, 0o644)
			_ = installCard(dataDir, serial, slot)
			_ = SaveState(dataDir, st.Set(serial, slot, Synced{SHA: srvMeta.SHA}))
			rep.Pulled = true
			rep.Notes = append(rep.Notes, Note{"Save conflict: server copy won, local backup kept", true})
			return true
		}
	}
	_ = installCard(dataDir, serial, slot)
	_ = SaveState(dataDir, st.Set(serial, slot, Synced{SHA: localHash}))
	rep.Notes = append(rep.Notes, Note{"Save conflict: local copy won", true})
	return true
}

// Push uploads changed working cards after the emulator exits.
func Push(c *api.Client, dataDir, serial string) Report {
	rep := Report{}
	st := LoadState(dataDir)
	dirty := false
	for _, slot := range []int{1, 2} {
		workHash, werr := HashFile(WorkingCard(dataDir, slot))
		if werr != nil {
			continue
		}
		if last, ok := st.Get(serial, slot); ok && last.SHA == workHash {
			continue
		}
		raw, err := os.ReadFile(WorkingCard(dataDir, slot))
		if err != nil {
			continue
		}
		meta, err := c.PutSave(serial, slot, raw)
		if err != nil {
			rep.Failed = true
			continue
		}
		_ = CopyFile(LocalCard(dataDir, serial, slot), WorkingCard(dataDir, slot))
		st = st.Set(serial, slot, Synced{SHA: meta.SHA})
		dirty, rep.Pushed = true, true
	}
	if dirty {
		_ = SaveState(dataDir, st)
	}
	switch {
	case rep.Pushed:
		rep.Notes = append(rep.Notes, Note{"Cloud saves synced", true})
	case rep.Failed:
		rep.Notes = append(rep.Notes, Note{"Save upload failed, kept locally", false})
	}
	return rep
}
