package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/x1nx3r/cache-22-client/internal/store"
)

func TestServerRegistry(t *testing.T) {
	loginSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/login" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"token":"tok-123","user":{"username":"u"}}`))
	}))
	defer loginSrv.Close()

	s := NewServerService(t.TempDir())

	servers, err := s.Servers()
	if err != nil || len(servers) != 0 {
		t.Fatalf("fresh registry = %v, %v", servers, err)
	}
	if active, _ := s.Active(); active != "" {
		t.Fatalf("fresh active = %q", active)
	}

	if err := s.AddServer("", loginSrv.URL, "u", "p"); err == nil {
		t.Error("empty name must fail")
	}
	if err := s.AddServer("a", loginSrv.URL, "", ""); err == nil {
		t.Error("missing credentials must fail")
	}
	if err := s.AddServer("a", loginSrv.URL+"/", "u", "p"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddServer("b", loginSrv.URL, "u", "p"); err == nil {
		t.Error("duplicate url must fail")
	}

	servers, _ = s.Servers()
	if len(servers) != 1 || servers[0].URL != loginSrv.URL {
		t.Errorf("servers = %+v (want normalized urls)", servers)
	}
	if servers[0].Token != "tok-123" || servers[0].Username != "u" {
		t.Errorf("credentials not stored: %+v", servers[0])
	}
	if active, _ := s.Active(); active != loginSrv.URL {
		t.Errorf("first added must become active, got %q", active)
	}

	entry, err := s.ActiveEntry()
	if err != nil || entry.Token != "tok-123" {
		t.Errorf("active entry = %+v, %v", entry, err)
	}

	s2 := NewServerService(filepath.Dir(s.path))
	servers2, err := s2.Servers()
	if err != nil || len(servers2) != 1 || servers2[0].Token != "tok-123" {
		t.Errorf("registry must persist tokens: %v, %v", servers2, err)
	}
}

func TestConnectionProbe(t *testing.T) {
	s := NewServerService(t.TempDir())
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Write([]byte(`{"status":"ok"}`))
		case "/v1/auth/login":
			w.Write([]byte(`{"token":"tok-123"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ok.Close()
	if err := s.TestConnection(ok.URL, "", ""); err != nil {
		t.Errorf("reachable server: %v", err)
	}
	if err := s.TestConnection(ok.URL, "u", "p"); err != nil {
		t.Errorf("valid credentials: %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer bad.Close()
	if err := s.TestConnection(bad.URL, "u", "wrong"); err == nil {
		t.Error("bad credentials must fail")
	}
	if err := s.TestConnection("http://127.0.0.1:1", "", ""); err == nil {
		t.Error("dead server must fail")
	}
}

func TestEmulatorIdleStatus(t *testing.T) {
	e := NewEmulatorService(NewServerService(t.TempDir()), t.TempDir(), "v2.9.30", false)
	st, err := e.EmuStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.Running {
		t.Error("fresh service must report idle")
	}
	if err := e.StopEmulator(); err != nil {
		t.Errorf("stop when idle must be nil: %v", err)
	}
	if e.Version() != "v2.9.30" {
		t.Errorf("version = %q", e.Version())
	}
	if e.DataDir() == "" {
		t.Error("datadir must not be empty")
	}
	tier, err := e.FetchTier()
	if err != nil {
		t.Fatal(err)
	}
	if tier.Probed || tier.FetchSize <= 0 || tier.PrefetchDepth <= 0 {
		t.Errorf("unprobed tier must carry defaults: %+v", tier)
	}
	if _, err := e.LastProbe(); err != nil {
		t.Errorf("last probe when none must not error: %v", err)
	}
}

func TestProbeNow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/games", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"games":[{"serial":"SLUS-1","title":"T","size_bytes":8}]}`))
	})
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v1/files/SLUS-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 0-7/8")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(make([]byte, 8))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	s := NewServerService(dir)
	f, _ := s.load()
	f.Servers = append(f.Servers, ServerEntry{Name: "t", URL: srv.URL, Username: "u", Token: "tok"})
	f.Active = srv.URL
	if err := s.save(f); err != nil {
		t.Fatal(err)
	}
	e := NewEmulatorService(s, dir, "v2.9.30", false)
	link, err := e.ProbeNow()
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if link.Class == "" {
		t.Errorf("probe must classify: %+v", link)
	}
	tier, err := e.FetchTier()
	if err != nil {
		t.Fatal(err)
	}
	if !tier.Probed || tier.Class != link.Class {
		t.Errorf("tier must reflect probe: %+v vs %+v", tier, link)
	}
}

func TestLibraryClean(t *testing.T) {
	dir := t.TempDir()
	l := NewLibraryService(NewServerService(dir), dir)
	if l.DataDir() != dir {
		t.Errorf("datadir = %q", l.DataDir())
	}
	gamesDir := filepath.Join(dir, "games")
	if err := os.MkdirAll(gamesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	iso, prog := store.Paths(gamesDir, "SLUS-1")
	for _, p := range []string{iso, prog} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Clean("SLUS-1"); err != nil {
		t.Fatalf("clean serial: %v", err)
	}
	if _, err := os.Stat(iso); !os.IsNotExist(err) {
		t.Error("iso must be gone after per-serial clean")
	}
	for _, p := range []string{iso, prog} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Clean("all"); err != nil {
		t.Fatalf("clean all: %v", err)
	}
	if _, err := os.Stat(gamesDir); !os.IsNotExist(err) {
		t.Error("games dir must be gone after clean all")
	}
}

func fakeAPIServer(t *testing.T, data []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"tok-123","user":{"username":"u"}}`))
	})
	mux.HandleFunc("/v1/games", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-123" {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"games":[{"serial":"G","title":"Game","size_bytes":` + fmt.Sprint(len(data)) + `}]}`))
	})
	mux.HandleFunc("/v1/games/G/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":2,"serial":"G","title":"Game","size_bytes":%d,"block_size":131072,"boot_ranges":[[0,1048576]],"file_url":"/v1/files/G","supported":true}`, len(data))
	})
	mux.HandleFunc("/v1/files/G", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "G.iso", time.Now(), bytes.NewReader(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func seedServer(t *testing.T, url string) *ServerService {
	t.Helper()
	s := NewServerService(t.TempDir())
	if err := s.AddServer("test", url, "u", "p"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLibraryGames(t *testing.T) {
	data := make([]byte, 2<<20)
	srv := fakeAPIServer(t, data)
	s := seedServer(t, srv.URL)
	lib := NewLibraryService(s, t.TempDir())

	games, err := lib.Games()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].Serial != "G" || games[0].Total != 2<<20 || games[0].Done != 0 {
		t.Errorf("games = %+v", games)
	}

	bare := NewLibraryService(NewServerService(t.TempDir()), t.TempDir())
	if _, err := bare.Games(); err != errNoServer {
		t.Errorf("no server err = %v", err)
	}
}

func TestDownloadFlow(t *testing.T) {
	data := make([]byte, 2<<20)
	for i := range data {
		data[i] = byte(i % 251)
	}
	srv := fakeAPIServer(t, data)
	dir := t.TempDir()
	s := seedServer(t, srv.URL)
	dl := NewDownloadService(s, dir)

	if err := dl.FetchBoot("G"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		st, err := dl.Status("G")
		if err != nil {
			t.Fatal(err)
		}
		if !st.Running && st.Done == 1<<20 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("boot fetch stuck: %+v", st)
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, err := os.ReadFile(filepath.Join(dir, "games", "G.iso"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range got[:1<<20] {
		if got[i] != data[i] {
			t.Fatalf("byte %d mismatch", i)
		}
	}
}

func TestImportBIOS(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "mybios.bin")
	if err := os.WriteFile(src, []byte("bios-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	em := NewEmulatorService(NewServerService(t.TempDir()), dataDir, "v", false)
	em.SetPicker(func() ([]string, error) { return []string{src}, nil })
	n, err := em.ImportBIOS()
	if err != nil || n != 1 {
		t.Fatalf("ImportBIOS = (%d, %v)", n, err)
	}
	if !em.BiosOK() {
		t.Error("BIOS must be detected after import")
	}
	installed, err := os.ReadFile(filepath.Join(dataDir, "PCSX2", "bios", "mybios.bin"))
	if err != nil || string(installed) != "bios-bytes" {
		t.Errorf("installed bytes wrong: %q, %v", installed, err)
	}

	emCancel := NewEmulatorService(NewServerService(t.TempDir()), t.TempDir(), "v", false)
	emCancel.SetPicker(func() ([]string, error) { return nil, nil })
	if n, err := emCancel.ImportBIOS(); n != 0 || err != nil {
		t.Errorf("cancel = (%d, %v), want (0, nil)", n, err)
	}

	emNoPicker := NewEmulatorService(NewServerService(t.TempDir()), t.TempDir(), "v", false)
	if _, err := emNoPicker.ImportBIOS(); err == nil {
		t.Error("missing picker must fail")
	}
}

func TestPlayNeedsBIOS(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := fakeAPIServer(t, make([]byte, 2<<20))
	s := seedServer(t, srv.URL)
	em := NewEmulatorService(s, t.TempDir(), "v2.9.30", false)

	if em.BiosOK() {
		t.Error("fresh HOME must have no BIOS")
	}
	if err := em.Play("G"); err != ErrNoBIOS {
		t.Errorf("Play without BIOS = %v, want ErrNoBIOS", err)
	}

	biosDir := filepath.Join(os.Getenv("HOME"), ".config", "PCSX2", "bios")
	if err := os.MkdirAll(biosDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(biosDir, "bios.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !em.BiosOK() {
		t.Error("BIOS file present, BiosOK must be true")
	}
}
