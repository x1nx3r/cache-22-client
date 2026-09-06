package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/store"
)

type GameInfo struct {
	Serial    string `json:"serial"`
	Title     string `json:"title"`
	Size      int64  `json:"size"`
	Supported bool   `json:"supported"`
	Done      int64  `json:"done"`
	Total     int64  `json:"total"`
	LocalPath string `json:"localPath"`
}

type LibraryService struct {
	servers *ServerService
	dataDir string
}

func NewLibraryService(servers *ServerService, dataDir string) *LibraryService {
	return &LibraryService{servers: servers, dataDir: dataDir}
}

func (l *LibraryService) client() (*api.Client, error) {
	entry, err := l.servers.ActiveEntry()
	if err != nil {
		return nil, err
	}
	return api.New(entry.URL, entry.Token), nil
}

func (l *LibraryService) Games() ([]GameInfo, error) {
	c, err := l.client()
	if err != nil {
		return nil, err
	}
	games, err := api.ListGames(c)
	if err != nil {
		return nil, err
	}
	out := make([]GameInfo, 0, len(games))
	for _, g := range games {
		m, err := api.GetManifest(c, g.Serial)
		if err != nil {
			return nil, err
		}
		done, total, path := l.progress(m)
		out = append(out, GameInfo{
			Serial: g.Serial, Title: g.Title, Size: g.SizeBytes,
			Supported: m.Supported, Done: done, Total: total, LocalPath: path,
		})
	}
	return out, nil
}

func (l *LibraryService) DataDir() string {
	return l.dataDir
}

func (l *LibraryService) Clean(serial string) error {
	gamesDir := filepath.Join(l.dataDir, "games")
	mntDir := filepath.Join(l.dataDir, "mnt")
	if serial == "all" {
		if entries, err := os.ReadDir(mntDir); err == nil {
			for _, entry := range entries {
				unmountGUI(filepath.Join(mntDir, entry.Name()))
			}
		}
		if err := os.RemoveAll(gamesDir); err != nil {
			return err
		}
		return os.RemoveAll(mntDir)
	}
	unmountGUI(filepath.Join(mntDir, mntName(serial)))
	iso, prog := store.Paths(gamesDir, serial)
	for _, p := range []string{iso, prog} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func unmountGUI(dir string) {
	if _, err := os.Stat(dir); err != nil {
		return
	}
	if err := exec.Command("fusermount3", "-u", dir).Run(); err != nil {
		_ = exec.Command("umount", "-l", dir).Run()
	}
}

func (l *LibraryService) Progress(serial string) (GameInfo, error) {
	c, err := l.client()
	if err != nil {
		return GameInfo{}, err
	}
	m, err := api.GetManifest(c, serial)
	if err != nil {
		return GameInfo{}, err
	}
	done, total, path := l.progress(m)
	return GameInfo{Serial: m.Serial, Title: m.Title, Size: m.SizeBytes,
		Supported: m.Supported, Done: done, Total: total, LocalPath: path}, nil
}

func (l *LibraryService) progress(m api.Manifest) (int64, int64, string) {
	s, err := store.Open(filepath.Join(l.dataDir, "games"), m.Serial, m.SizeBytes, 0)
	if err != nil {
		return 0, m.SizeBytes, ""
	}
	defer s.Close()
	return s.DoneBytes(), m.SizeBytes, s.Path
}
