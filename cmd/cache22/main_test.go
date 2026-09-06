package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cache-22/cache-22-client/internal/config"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{DataDir: t.TempDir()}
}

func seedGame(t *testing.T, cfg config.Config, serial string) {
	t.Helper()
	dir := filepath.Join(cfg.DataDir, "games")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{serial + ".iso", serial + ".progress.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanOne(t *testing.T) {
	cfg := testConfig(t)
	seedGame(t, cfg, "G")
	if err := cmdClean(cfg, []string{"G"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "games", "G.iso")); !os.IsNotExist(err) {
		t.Error("iso must be gone")
	}
	if err := cmdClean(cfg, []string{"G"}); err == nil {
		t.Error("second clean must report nothing cached")
	}
	if err := cmdClean(cfg, nil); err == nil {
		t.Error("missing arg must fail")
	}
}

func TestCleanAll(t *testing.T) {
	cfg := testConfig(t)
	seedGame(t, cfg, "A")
	seedGame(t, cfg, "B")
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "mnt", "A"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmdClean(cfg, []string{"all"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "games")); !os.IsNotExist(err) {
		t.Error("games dir must be gone")
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "mnt")); !os.IsNotExist(err) {
		t.Error("mnt dir must be gone")
	}
}
