package profile

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type entry struct {
	Off int64 `json:"off"`
	Len int   `json:"len"`
	T   int64 `json:"t"`
}

type Tracer struct {
	mu    sync.Mutex
	w     *bufio.Writer
	f     *os.File
	count int
}

func TracePath(dataDir, serial string) string {
	return filepath.Join(dataDir, "profiles", safeName(serial)+".trace.jsonl")
}

func ProfilePath(dataDir, serial string) string {
	return filepath.Join(dataDir, "profiles", safeName(serial)+".profile.json")
}

func safeName(serial string) string {
	out := ""
	for _, r := range serial {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out += string(r)
		} else {
			out += "_"
		}
	}
	if out == "" {
		out = "game"
	}
	return out
}

func Start(dataDir, serial string) (*Tracer, error) {
	path := TracePath(dataDir, serial)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Tracer{w: bufio.NewWriterSize(f, 64<<10), f: f}, nil
}

func (t *Tracer) Log(off int64, length int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	raw, _ := json.Marshal(entry{Off: off, Len: length, T: time.Now().UnixMilli()})
	t.w.Write(raw)
	t.w.WriteByte('\n')
	t.count++
	if t.count%1000 == 0 {
		t.w.Flush()
	}
}

func (t *Tracer) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.w.Flush(); err != nil {
		t.f.Close()
		return err
	}
	return t.f.Close()
}

type ChunkStat struct {
	Index int64 `json:"index"`
	Count int64 `json:"count"`
	First int64 `json:"first"`
}

type Heatmap struct {
	Serial   string      `json:"serial"`
	Sessions int         `json:"sessions"`
	Chunks   []ChunkStat `json:"chunks"`
}

func Aggregate(dataDir, serial string, chunkLen int64) (Heatmap, error) {
	hm, _ := loadProfile(ProfilePath(dataDir, serial))
	hm.Serial = serial

	raw, err := os.ReadFile(TracePath(dataDir, serial))
	if err != nil {
		return hm, err
	}
	byChunk := map[int64]*ChunkStat{}
	for _, s := range hm.Chunks {
		c := s
		byChunk[s.Index] = &c
	}
	order := int64(len(byChunk))
	for _, line := range splitLines(raw) {
		var e entry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		i := e.Off / chunkLen
		c, ok := byChunk[i]
		if !ok {
			c = &ChunkStat{Index: i, First: order}
			order++
			byChunk[i] = c
		}
		c.Count++
	}
	hm.Chunks = hm.Chunks[:0]
	for _, c := range byChunk {
		hm.Chunks = append(hm.Chunks, *c)
	}
	sort.Slice(hm.Chunks, func(a, b int) bool { return hm.Chunks[a].First < hm.Chunks[b].First })
	hm.Sessions++
	return hm, nil
}

func Save(dataDir, serial string, hm Heatmap) error {
	raw, err := json.MarshalIndent(hm, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProfilePath(dataDir, serial), raw, 0o644)
}

func loadProfile(path string) (Heatmap, error) {
	var hm Heatmap
	raw, err := os.ReadFile(path)
	if err != nil {
		return hm, err
	}
	return hm, json.Unmarshal(raw, &hm)
}

func HotSet(hm Heatmap, n int) []int64 {
	if n > len(hm.Chunks) {
		n = len(hm.Chunks)
	}
	out := make([]int64, 0, n)
	for _, c := range hm.Chunks[:n] {
		out = append(out, c.Index)
	}
	return out
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				out = append(out, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		out = append(out, raw[start:])
	}
	return out
}
