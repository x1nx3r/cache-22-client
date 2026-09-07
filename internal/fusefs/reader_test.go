package fusefs

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/store"
)

func testSetup(t *testing.T, data []byte, hits *atomic.Int64) *api.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api.New(srv.URL, "")
}

func patternData(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func openStore(t *testing.T, n int64) *store.Sparse {
	t.Helper()
	st, err := store.Open(t.TempDir(), "G", n, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestReadAtFetchesExactRange(t *testing.T) {
	data := patternData(1 << 20)
	var hits atomic.Int64
	client := testSetup(t, data, &hits)
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	got := make([]byte, 100)
	n, err := r.ReadAt(got, 5000)
	if err != nil || n != 100 {
		t.Fatalf("ReadAt = (%d, %v)", n, err)
	}
	for i := range got {
		if got[i] != data[5000+i] {
			t.Fatalf("byte %d mismatch", i)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("requests = %d, want exactly 1", hits.Load())
	}
	if !st.HasBlock(5000 / store.DefaultBlockLen) {
		t.Error("block must be cached after read")
	}
}

func TestReadAtSingleflight(t *testing.T) {
	data := patternData(1 << 20)
	var hits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(100 * time.Millisecond)
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := api.New(srv.URL, "")
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	if _, err := r.ReadAt(make([]byte, 10), 2*131072); err != nil {
		t.Fatal(err)
	}
	hits.Store(0)

	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, err := r.ReadAt(make([]byte, 100), 0)
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("range fetched %d times, want 1", hits.Load())
	}
}

func TestReadPastEnd(t *testing.T) {
	data := patternData(100)
	client := testSetup(t, data, nil)
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	n, err := r.ReadAt(make([]byte, 10), 1000)
	if n != 0 || err != nil {
		t.Errorf("past-end = (%d, %v), want (0, nil)", n, err)
	}
}

func TestReadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	t.Cleanup(srv.Close)
	client := api.New(srv.URL, "")
	st := openStore(t, 1<<20)
	r := NewReader(client, st, 1<<20, "G", LANConfig())

	if _, err := r.ReadAt(make([]byte, 10), 0); err == nil {
		t.Error("want error, got nil")
	}
}

func TestMountRead(t *testing.T) {
	if _, err := os.Stat("/dev/fuse"); err != nil {
		t.Skip("no /dev/fuse")
	}
	data := patternData(2<<20 + 12345)
	var hits atomic.Int64
	client := testSetup(t, data, &hits)
	st := openStore(t, int64(len(data)))

	mnt := filepath.Join(t.TempDir(), "mnt")
	if err := os.Mkdir(mnt, 0o755); err != nil {
		t.Fatal(err)
	}
	server, err := Mount(mnt, NewReader(client, st, int64(len(data)), "G", LANConfig()), int64(len(data)))
	if err != nil {
		t.Skipf("mount failed: %v", err)
	}
	t.Cleanup(func() { server.Unmount() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(mnt, "game.iso")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mount never appeared")
		}
		time.Sleep(50 * time.Millisecond)
	}

	f, err := os.Open(filepath.Join(mnt, "game.iso"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if st2, err := f.Stat(); err != nil || st2.Size() != int64(len(data)) {
		t.Fatalf("stat size wrong: %v, %v", st2.Size(), err)
	}
	got := make([]byte, 0, len(data))
	buf := make([]byte, 1<<20)
	for {
		n, err := f.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	if len(got) != len(data) {
		t.Fatalf("read %d bytes, want %d", len(got), len(data))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("byte %d mismatch", i)
		}
	}
	if hits.Load() == 0 {
		t.Error("expected server fetches")
	}
}

func TestPrefetchSequential(t *testing.T) {
	data := patternData(16 << 20)
	var hits atomic.Int64
	client := testSetup(t, data, &hits)
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	if _, err := r.ReadAt(make([]byte, 100), 0); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !st.HasBlock(8) {
		if time.Now().After(deadline) {
			t.Fatal("prefetch never filled block 8")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("requests = %d, want 2 (demand + one coalesced prefetch)", got)
	}
}

func TestPrefetchSkipsSeeks(t *testing.T) {
	data := patternData(16 << 20)
	var hits atomic.Int64
	client := testSetup(t, data, &hits)
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	if _, err := r.ReadAt(make([]byte, 100), 0); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !st.HasBlock(8) {
		if time.Now().After(deadline) {
			t.Fatal("setup prefetch never settled")
		}
		time.Sleep(20 * time.Millisecond)
	}
	hits.Store(0)
	if _, err := r.ReadAt(make([]byte, 100), 10<<20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if got := hits.Load(); got != 1 {
		t.Errorf("requests = %d, want 1 (seek fetches only itself)", got)
	}
	if st.HasBlock(10<<20/131072 + 8) {
		t.Error("seek must not prefetch ahead")
	}
}

func TestConfigForRTT(t *testing.T) {
	cases := []struct {
		rtt       float64
		fetchSize int64
		depth     int64
	}{
		{0, 128 << 10, 8},
		{4.9, 128 << 10, 8},
		{5, 1 << 20, 32},
		{49.9, 1 << 20, 32},
		{50, 2 << 20, 64},
		{300, 2 << 20, 64},
	}
	for _, c := range cases {
		got := ConfigForRTT(c.rtt)
		if got.FetchSize != c.fetchSize || got.PrefetchDepth != c.depth {
			t.Errorf("rtt %.1f: got %+v", c.rtt, got)
		}
	}
}

func TestEnsureRoundsUpToFetchSize(t *testing.T) {
	data := patternData(4 << 20)
	var got [][2]int64
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		got = append(got, [2]int64{start, end + 1})
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := api.New(srv.URL, "")
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", ConfigForRTT(20))

	if _, err := r.ReadAt(make([]byte, 100), 5000); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != [2]int64{0, 1 << 20} {
		t.Errorf("server saw %v, want one 1MB range from block start", got)
	}
}

func TestReadMidBlockThenHead(t *testing.T) {
	data := patternData(1 << 20)
	var hits atomic.Int64
	client := testSetup(t, data, &hits)
	st := openStore(t, int64(len(data)))
	r := NewReader(client, st, int64(len(data)), "G", LANConfig())

	if _, err := r.ReadAt(make([]byte, 100), 5000); err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 5000)
	if _, err := r.ReadAt(head, 0); err != nil {
		t.Fatal(err)
	}
	for i := range head {
		if head[i] != data[i] {
			t.Fatalf("byte %d = %d, want %d (block head must be fetched, not sparse zeros)", i, head[i], data[i])
		}
	}
	if hits.Load() != 1 {
		t.Errorf("requests = %d, want 1 (fetch must start at block start, covering the head)", hits.Load())
	}
}
