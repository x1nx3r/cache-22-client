package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const DefaultBlockLen = 128 << 10

const legacyChunkLen = 1 << 20

type Sparse struct {
	Path     string
	Size     int64
	BlockLen int64
	file     *os.File
	mu       sync.RWMutex
	blocks   map[int64]bool
	progPath string
}

func filename(dir, serial string) (string, string) {
	safe := strings.ReplaceAll(strings.ReplaceAll(serial, "/", "_"), "\\", "_")
	return filepath.Join(dir, safe+".iso"), filepath.Join(dir, safe+".progress.json")
}

func Paths(dir, serial string) (iso, progress string) {
	return filename(dir, serial)
}

func Open(dir, serial string, size, blockLen int64) (*Sparse, error) {
	if blockLen <= 0 {
		blockLen = DefaultBlockLen
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path, prog := filename(dir, serial)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if st.Size() != size {
		if err := f.Truncate(size); err != nil {
			f.Close()
			return nil, fmt.Errorf("truncate: %w", err)
		}
	}
	s := &Sparse{Path: path, Size: size, BlockLen: blockLen, file: f, blocks: map[int64]bool{}, progPath: prog}
	s.loadProgress()
	return s, nil
}

type progressFile struct {
	Version int     `json:"v"`
	Blocks  []int64 `json:"blocks"`
}

func (s *Sparse) loadProgress() {
	raw, err := os.ReadFile(s.progPath)
	if err != nil {
		return
	}
	var v2 progressFile
	if json.Unmarshal(raw, &v2) == nil && v2.Version == 2 {
		for _, i := range v2.Blocks {
			s.blocks[i] = true
		}
		return
	}
	var v1 []int64
	if json.Unmarshal(raw, &v1) == nil {
		perChunk := legacyChunkLen / s.BlockLen
		for _, c := range v1 {
			for b := c * perChunk; b < (c+1)*perChunk; b++ {
				s.blocks[b] = true
			}
		}
	}
}

func (s *Sparse) saveProgress() error {
	done := make([]int64, 0, len(s.blocks))
	for i := range s.blocks {
		done = append(done, i)
	}
	raw, err := json.Marshal(progressFile{Version: 2, Blocks: done})
	if err != nil {
		return err
	}
	return os.WriteFile(s.progPath, raw, 0o644)
}

func (s *Sparse) BlockCount() int64 {
	n := s.Size / s.BlockLen
	if s.Size%s.BlockLen != 0 {
		n++
	}
	return n
}

func (s *Sparse) blockRange(off, length int64) (int64, int64) {
	return off / s.BlockLen, (off + length - 1) / s.BlockLen
}

func (s *Sparse) HasBlock(i int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.blocks[i]
}

func (s *Sparse) ReadAt(p []byte, off int64) (int, error) {
	return s.file.ReadAt(p, off)
}

// WriteRange records a fetched range and marks its blocks present.
// off must start at a block boundary so the marked blocks are fully
// written; MissingRanges only produces such offsets.
func (s *Sparse) WriteRange(off int64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.file.WriteAt(data, off); err != nil {
		return err
	}
	lo, hi := s.blockRange(off, int64(len(data)))
	for i := lo; i <= hi; i++ {
		s.blocks[i] = true
	}
	return s.saveProgress()
}

// MissingRanges returns the sub-ranges of [off, off+length) whose blocks
// are not fetched yet. Ranges always start at a block boundary so that
// fetching them makes the marked blocks fully present; the final range is
// clamped at the end of the request and the caller rounds it up.
func (s *Sparse) MissingRanges(off, length int64) [][2]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lo, hi := s.blockRange(off, length)
	var out [][2]int64
	var cur [2]int64
	open := false
	for i := lo; i <= hi; i++ {
		if s.blocks[i] {
			if open {
				out = append(out, cur)
				open = false
			}
			continue
		}
		start := i * s.BlockLen
		end := start + s.BlockLen
		if i == hi && off+length < end {
			end = off + length
		}
		if !open {
			cur = [2]int64{start, end}
			open = true
		} else {
			cur[1] = end
		}
	}
	if open {
		out = append(out, cur)
	}
	return out
}

func (s *Sparse) DoneBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for i := range s.blocks {
		start := i * s.BlockLen
		if start >= s.Size {
			continue
		}
		end := start + s.BlockLen
		if end > s.Size {
			end = s.Size
		}
		n += end - start
	}
	return n
}

func (s *Sparse) Close() error {
	return s.file.Close()
}
