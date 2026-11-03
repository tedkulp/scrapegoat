package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

func writePlatformsCache(t *testing.T, dir string, fetchedAt time.Time) {
	t.Helper()
	cache := PlatformsCache{
		FetchedAt: fetchedAt,
		Platforms: map[string]Platform{"nes": {ID: 3, Name: "NES"}},
	}
	data, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "platforms.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubSystemsList(t *testing.T, systems []scraper.System, calls *int) {
	t.Helper()
	orig := fetchSystemsList
	fetchSystemsList = func(scraper.Credentials) ([]scraper.System, error) {
		*calls++
		return systems, nil
	}
	t.Cleanup(func() { fetchSystemsList = orig })
}

func TestGetPlatform_ReadsCacheFromGivenDir(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{}
	cfg.Output.CacheDir = filepath.Join(root, "from-config")
	given := filepath.Join(root, "given")
	writePlatformsCache(t, given, time.Now())

	calls := 0
	stubSystemsList(t, nil, &calls)

	p, err := cfg.GetPlatform("NES", given, 12)
	if err != nil {
		t.Fatalf("GetPlatform: %v", err)
	}
	if p.ID != 3 {
		t.Fatalf("platform ID = %d, want 3", p.ID)
	}
	if calls != 0 {
		t.Fatalf("API called %d times, want 0", calls)
	}
	if _, err := os.Stat(cfg.Output.CacheDir); !os.IsNotExist(err) {
		t.Fatalf("config cache dir was touched: %v", err)
	}
}

func TestGetPlatform_WritesCacheToGivenDir(t *testing.T) {
	root := t.TempDir()
	cfg := &Config{}
	cfg.Output.CacheDir = filepath.Join(root, "from-config")
	given := filepath.Join(root, "given")
	// Older than the 2-hour lifetime passed in, so it must be refetched
	// and rewritten in the given dir.
	writePlatformsCache(t, given, time.Now().Add(-3*time.Hour))

	calls := 0
	stubSystemsList(t, []scraper.System{
		{ID: 4, Names: []scraper.Name{{Region: "us", Text: "Super Nintendo"}}},
	}, &calls)

	p, err := cfg.GetPlatform("supernintendo", given, 2)
	if err != nil {
		t.Fatalf("GetPlatform: %v", err)
	}
	if p.ID != 4 || calls != 1 {
		t.Fatalf("platform = %+v, calls = %d", p, calls)
	}
	cache, err := loadPlatformsCache(filepath.Join(given, "platforms.json"))
	if err != nil {
		t.Fatalf("reading rewritten cache: %v", err)
	}
	if _, ok := cache.FindByName("supernintendo"); !ok {
		t.Fatalf("rewritten cache lacks the fetched platform: %+v", cache.Platforms)
	}
	if _, err := os.Stat(cfg.Output.CacheDir); !os.IsNotExist(err) {
		t.Fatalf("config cache dir was touched: %v", err)
	}
}

func TestGetPlatform_ConfigEntryWins(t *testing.T) {
	cfg := &Config{Platforms: map[string]Platform{"nes": {ID: 99}}}
	calls := 0
	stubSystemsList(t, nil, &calls)
	p, err := cfg.GetPlatform("nes", t.TempDir(), 12)
	if err != nil || p.ID != 99 || calls != 0 {
		t.Fatalf("p=%+v err=%v calls=%d", p, err, calls)
	}
}
