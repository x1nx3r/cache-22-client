package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/x1nx3r/cache-22-client/internal/api"
	"github.com/x1nx3r/cache-22-client/internal/save"
)

func fakeSaveServer(t *testing.T, store map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/saves/", func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		switch r.Method {
		case "HEAD":
			raw, ok := store[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("X-SHA256", "srv")
			w.WriteHeader(http.StatusOK)
			_ = raw
		case "GET":
			raw, ok := store[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("X-SHA256", "srv")
			w.Write(raw)
		case "PUT":
			rest, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read: %v", err)
			}
			store[r.URL.Path] = rest
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"sha256":"srv2","size":4,"updated_at":"2026-01-01T00:00:00Z"}`))
		default:
			_ = key
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testEmuService(dir string) *EmulatorService {
	return NewEmulatorService(NewServerService(dir), dir, "v2.9.30", false)
}

func TestPrepareAdoptsWorkingCard(t *testing.T) {
	dir := t.TempDir()
	srv := fakeSaveServer(t, map[string][]byte{})
	c := api.New(srv.URL, "tok")
	e := testEmuService(dir)

	// No library card, but a working card from a first boot: adopt it.
	work := save.WorkingCard(dir, 1)
	if err := os.MkdirAll(filepath.Dir(work), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, []byte("card-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.prepareSaves(c, "G")
	lib, err := os.ReadFile(save.LocalCard(dir, "G", 1))
	if err != nil || string(lib) != "card-bytes" {
		t.Errorf("library card = %q, %v", lib, err)
	}
	st := save.LoadState(dir)
	if s, ok := st.Get("G", 1); !ok || s.SHA == "" {
		t.Errorf("synced state = %+v", st)
	}
}

func TestPreparePullsServer(t *testing.T) {
	dir := t.TempDir()
	srv := fakeSaveServer(t, map[string][]byte{"/v1/saves/G/1": []byte("srv-card")})
	c := api.New(srv.URL, "tok")
	e := testEmuService(dir)

	e.prepareSaves(c, "G")
	lib, err := os.ReadFile(save.LocalCard(dir, "G", 1))
	if err != nil || string(lib) != "srv-card" {
		t.Errorf("library card = %q, %v", lib, err)
	}
	work, err := os.ReadFile(save.WorkingCard(dir, 1))
	if err != nil || string(work) != "srv-card" {
		t.Errorf("working card = %q, %v", work, err)
	}
	if e.saveSeq == 0 {
		t.Error("pull must record a save notice")
	}
}

func TestPushUploadsChanges(t *testing.T) {
	dir := t.TempDir()
	backing := map[string][]byte{}
	srv := fakeSaveServer(t, backing)
	c := api.New(srv.URL, "tok")
	e := testEmuService(dir)

	// Seed synced state, then modify the working card.
	lib := save.LocalCard(dir, "G", 2)
	if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lib, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := save.WorkingCard(dir, 2)
	if err := os.MkdirAll(filepath.Dir(work), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	h1, _ := save.HashFile(work)
	st := save.LoadState(dir).Set("G", 2, save.Synced{SHA: h1})
	if err := save.SaveState(dir, st); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, []byte("v2!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.pushSaves(c, "G")
	got, ok := backing["/v1/saves/G/2"]
	if !ok || string(got) != "v2!!" {
		t.Errorf("server got %q, present=%v", got, ok)
	}
	st2 := save.LoadState(dir)
	if s, ok := st2.Get("G", 2); !ok || s.SHA != "srv2" {
		t.Errorf("synced = %+v", st2)
	}
	if e.saveSeq == 0 || !e.saveOk {
		t.Errorf("push must note success: seq=%d ok=%v msg=%q", e.saveSeq, e.saveOk, e.saveMsg)
	}
}
