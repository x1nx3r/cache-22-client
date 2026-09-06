package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL    string
	DataDir      string
	PCSX2Version string
	Token        string
	Profiling    bool
}

func New() Config {
	home, _ := os.UserHomeDir()
	dataDir := os.Getenv("CACHE22_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share", "cache22")
	}
	return Config{
		ServerURL:    envOr("CACHE22_SERVER", "http://localhost:8080"),
		DataDir:      dataDir,
		PCSX2Version: envOr("CACHE22_PCSX2_VERSION", "v2.9.30"),
		Token:        os.Getenv("CACHE22_TOKEN"),
		Profiling:    envOr("CACHE22_PROFILE", "1") == "1",
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
