package syncer

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cache-22/cache-22-client/internal/api"
	"github.com/cache-22/cache-22-client/internal/store"
)

func patternData(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func testClient(t *testing.T, data []byte, ranges *[][2]int64) *api.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		if ranges != nil {
			*ranges = append(*ranges, [2]int64{start, end + 1})
		}
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api.New(srv.URL, "")
}

func TestFillRangesExact(t *testing.T) {
	data := patternData(1 << 20)
	var mu sync.Mutex
	var got [][2]int64
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		var start, end int64
		fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
		mu.Lock()
		got = append(got, [2]int64{start, end + 1})
		mu.Unlock()
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := api.New(srv.URL, "")
	st, _ := store.Open(t.TempDir(), "G", int64(len(data)), 0)
	defer st.Close()

	if err := FillRanges(client, "G", st, [][2]int64{{100, 200}, {5000, 9000}}, 2); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("server saw %d ranges, want 2", len(got))
	}
	seen := map[[2]int64]bool{}
	for _, r := range got {
		seen[r] = true
	}
	if !seen[[2]int64{100, 200}] || !seen[[2]int64{5000, 9000}] {
		t.Errorf("server saw ranges %v, want exact requests", got)
	}
	back := make([]byte, 100)
	if _, err := st.ReadAt(back, 100); err != nil {
		t.Fatal(err)
	}
	for i := range back {
		if back[i] != data[100+i] {
			t.Fatalf("byte %d mismatch", i)
		}
	}
}

func TestWholeImageSegments(t *testing.T) {
	segs := WholeImage(100<<20, 32<<20)
	if len(segs) != 4 || segs[0] != [2]int64{0, 32 << 20} || segs[3][1] != 100<<20 {
		t.Errorf("segs = %v", segs)
	}
	segs = WholeImage(10, 0)
	if len(segs) != 1 || segs[0] != [2]int64{0, 10} {
		t.Errorf("small segs = %v", segs)
	}
}

func TestFillError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusGone)
	}))
	t.Cleanup(srv.Close)
	client := api.New(srv.URL, "")
	st, _ := store.Open(t.TempDir(), "G", 1<<20, 0)
	defer st.Close()
	if err := FillRanges(client, "G", st, [][2]int64{{0, 100}}, 1); err == nil {
		t.Error("want error, got nil")
	}
}

func TestPreloadRanges(t *testing.T) {
	boot := [][2]int64{{0, 1 << 20}}
	if got := PreloadRanges(4<<30, boot, "lan"); len(got) != 1 || got[0] != boot[0] {
		t.Errorf("lan = %v", got)
	}
	if got := PreloadRanges(4<<30, boot, "mid"); len(got) != 2 || got[1] != [2]int64{0, 256 << 20} {
		t.Errorf("mid = %v", got)
	}
	if got := PreloadRanges(4<<30, boot, "slow"); len(got) != 2 || got[1] != [2]int64{0, 1 << 30} {
		t.Errorf("slow = %v", got)
	}
	if got := PreloadRanges(100, boot, "mid"); len(got) != 1 || got[0] != [2]int64{0, 100} {
		t.Errorf("small image = %v", got)
	}
}
