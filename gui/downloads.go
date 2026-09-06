package main

import (
	"path/filepath"
	"sync"

	"github.com/cache-22/cache-22-client/internal/api"
	"github.com/cache-22/cache-22-client/internal/store"
	"github.com/cache-22/cache-22-client/internal/syncer"
)

type DownloadStatus struct {
	Running bool  `json:"running"`
	Done    int64 `json:"done"`
	Total   int64 `json:"total"`
}

type DownloadService struct {
	servers *ServerService
	dataDir string
	mu      sync.Mutex
	running map[string]bool
}

func NewDownloadService(servers *ServerService, dataDir string) *DownloadService {
	return &DownloadService{servers: servers, dataDir: dataDir, running: map[string]bool{}}
}

func (d *DownloadService) client() (*api.Client, error) {
	entry, err := d.servers.ActiveEntry()
	if err != nil {
		return nil, err
	}
	return api.New(entry.URL, entry.Token), nil
}

func (d *DownloadService) FetchBoot(serial string) error {
	return d.start(serial, true)
}

func (d *DownloadService) SyncFull(serial string) error {
	return d.start(serial, false)
}

func (d *DownloadService) start(serial string, bootOnly bool) error {
	d.mu.Lock()
	if d.running[serial] {
		d.mu.Unlock()
		return nil
	}
	d.running[serial] = true
	d.mu.Unlock()

	go func() {
		defer func() {
			d.mu.Lock()
			delete(d.running, serial)
			d.mu.Unlock()
		}()
		_ = d.run(serial, bootOnly)
	}()
	return nil
}

func (d *DownloadService) run(serial string, bootOnly bool) error {
	c, err := d.client()
	if err != nil {
		return err
	}
	m, err := api.GetManifest(c, serial)
	if err != nil {
		return err
	}
	s, err := store.Open(filepath.Join(d.dataDir, "games"), m.Serial, m.SizeBytes, 0)
	if err != nil {
		return err
	}
	defer s.Close()
	var ranges [][2]int64
	if bootOnly {
		for _, r := range m.BootRanges {
			if r[0] >= m.SizeBytes {
				continue
			}
			end := r[1]
			if end > m.SizeBytes {
				end = m.SizeBytes
			}
			ranges = append(ranges, [2]int64{r[0], end})
		}
	} else {
		ranges = syncer.WholeImage(m.SizeBytes, 0)
	}
	var want [][2]int64
	for _, r := range ranges {
		want = append(want, s.MissingRanges(r[0], r[1]-r[0])...)
	}
	return syncer.FillRanges(c, m.Serial, s, want, 4)
}

func (d *DownloadService) Status(serial string) (DownloadStatus, error) {
	c, err := d.client()
	if err != nil {
		return DownloadStatus{}, err
	}
	m, err := api.GetManifest(c, serial)
	if err != nil {
		return DownloadStatus{}, err
	}
	s, err := store.Open(filepath.Join(d.dataDir, "games"), m.Serial, m.SizeBytes, 0)
	if err != nil {
		return DownloadStatus{}, err
	}
	defer s.Close()
	d.mu.Lock()
	running := d.running[serial]
	d.mu.Unlock()
	return DownloadStatus{Running: running, Done: s.DoneBytes(), Total: m.SizeBytes}, nil
}
