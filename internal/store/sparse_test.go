package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOpenCreatesSparse(t *testing.T) {
	s, err := Open(t.TempDir(), "G", 3<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.BlockLen != DefaultBlockLen {
		t.Errorf("BlockLen = %d", s.BlockLen)
	}
	if s.BlockCount() != 24 {
		t.Errorf("BlockCount = %d, want 24", s.BlockCount())
	}
	st, _ := os.Stat(s.Path)
	if st.Size() != 3<<20 {
		t.Errorf("size = %d", st.Size())
	}
	if len(s.MissingRanges(0, 3<<20)) != 1 {
		t.Error("everything must start missing")
	}
}

func TestWriteRangeAndResume(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, "G", 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello-range")
	if err := s.WriteRange(500, data); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(dir, "G", 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if !s2.HasBlock(0) {
		t.Error("block 0 must survive reopen")
	}
	if len(s2.MissingRanges(0, 1<<20)) == 0 {
		t.Error("most of the file must still be missing")
	}
	got := make([]byte, len(data))
	if _, err := s2.ReadAt(got, 500); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Error("range bytes not at right offset")
	}
}

func TestMissingRangesCoalesces(t *testing.T) {
	s, err := Open(t.TempDir(), "G", 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.WriteRange(0, make([]byte, 100)); err != nil {
		t.Fatal(err)
	}
	missing := s.MissingRanges(0, 1<<20)
	if len(missing) != 1 {
		t.Fatalf("missing = %v, want 1 coalesced range", missing)
	}
	if missing[0][0] != DefaultBlockLen || missing[0][1] != 1<<20 {
		t.Errorf("missing = %v", missing)
	}
}

func TestDoneBytes(t *testing.T) {
	s, err := Open(t.TempDir(), "G", 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.WriteRange(0, make([]byte, 100)); err != nil {
		t.Fatal(err)
	}
	if got := s.DoneBytes(); got != 1000 && got != DefaultBlockLen {
		t.Errorf("DoneBytes = %d (block granularity)", got)
	}
}

func TestLegacyProgressCompat(t *testing.T) {
	dir := t.TempDir()
	iso, prog := Paths(dir, "G")
	_ = iso
	raw, _ := json.Marshal([]int64{0, 2})
	if err := os.WriteFile(prog, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir, "G", 3*(1<<20), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	perChunk := int64((1 << 20) / DefaultBlockLen)
	for b := int64(0); b < perChunk; b++ {
		if !s.HasBlock(b) {
			t.Errorf("block %d must come from legacy chunk 0", b)
		}
	}
	if s.HasBlock(perChunk) {
		t.Error("chunk 1 was not in legacy progress")
	}
	if got := s.DoneBytes(); got != 2*(1<<20) {
		t.Errorf("DoneBytes = %d, want 2MB", got)
	}
}

func TestPathsSanitized(t *testing.T) {
	iso, prog := Paths(t.TempDir(), "../evil")
	if filepath.Base(iso) != ".._evil.iso" || filepath.Base(prog) != ".._evil.progress.json" {
		t.Errorf("paths = %q %q", iso, prog)
	}
}

func TestSparseConcurrent(t *testing.T) {
	s, err := Open(t.TempDir(), "G", 32*DefaultBlockLen, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var wg sync.WaitGroup
	for b := int64(0); b < 16; b++ {
		wg.Add(1)
		go func(b int64) {
			defer wg.Done()
			off := b * DefaultBlockLen
			if err := s.WriteRange(off, make([]byte, DefaultBlockLen)); err != nil {
				t.Errorf("write %d: %v", b, err)
			}
		}(b)
		wg.Add(1)
		go func(b int64) {
			defer wg.Done()
			_ = s.MissingRanges(b*DefaultBlockLen, DefaultBlockLen)
			_ = s.HasBlock(b)
			_ = s.DoneBytes()
		}(b)
	}
	wg.Wait()
	if got := s.DoneBytes(); got != 16*DefaultBlockLen {
		t.Errorf("DoneBytes = %d, want %d", got, 16*DefaultBlockLen)
	}
}
