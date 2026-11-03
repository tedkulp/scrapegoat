package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/scrapegoat/pkg/config"
)

func TestResolveCacheDir_FlagReachesPlatformCache(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Output.CacheDir = filepath.Join(root, "from-config")

	dir := resolveCacheDir(cfg, filepath.Join(root, "{platform}-cache"), "nes")

	want := filepath.Join(root, "nes-cache")
	if dir != want {
		t.Fatalf("resolveCacheDir = %q, want %q", dir, want)
	}

	// A fresh platforms.json in the flag's directory must satisfy the
	// platform lookup without any request to systemesListe.php.
	cache := config.PlatformsCache{
		FetchedAt: time.Now(),
		Platforms: map[string]config.Platform{"nes": {ID: 3, Name: "NES"}},
	}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(want, "platforms.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	platform, err := cfg.GetPlatform("nes")
	if err != nil {
		t.Fatalf("GetPlatform: %v", err)
	}
	if platform.ID != 3 {
		t.Fatalf("platform ID = %d, want 3", platform.ID)
	}
	if _, err := os.Stat(filepath.Join(root, "from-config")); !os.IsNotExist(err) {
		t.Fatalf("config cache dir was touched: %v", err)
	}
}

func TestResolveCacheDir_NoFlagUsesConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Output.CacheDir = "/from/config"

	if dir := resolveCacheDir(cfg, "", "nes"); dir != "/from/config" {
		t.Fatalf("resolveCacheDir = %q, want /from/config", dir)
	}
	if cfg.Output.CacheDir != "/from/config" {
		t.Fatalf("cfg.Output.CacheDir = %q, want /from/config", cfg.Output.CacheDir)
	}
}
