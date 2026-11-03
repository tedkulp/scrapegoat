package main

import (
	"strings"
	"testing"

	"github.com/tedkulp/scrapegoat/pkg/config"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Output.CacheDir = "/cfg/cache"
	return cfg
}

func validFlags() scrapeFlags {
	return scrapeFlags{
		platform:   "nes",
		romDir:     "/roms/nes",
		workers:    4,
		cacheHours: 12,
	}
}

func TestResolveScrapeSettings_DirsDefaultToROMDir(t *testing.T) {
	s, err := resolveScrapeSettings(validFlags(), testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.GamelistDir != "/roms/nes" {
		t.Errorf("GamelistDir = %q, want /roms/nes", s.GamelistDir)
	}
	if s.MediaRootDir != "/roms/nes" {
		t.Errorf("MediaRootDir = %q, want /roms/nes", s.MediaRootDir)
	}
}

func TestResolveScrapeSettings_ExplicitDirsKept(t *testing.T) {
	f := validFlags()
	f.gamelistDir = "/out/gl"
	f.mediaRootDir = "/out/media"
	f.cacheDir = "/out/cache"
	s, err := resolveScrapeSettings(f, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.GamelistDir != "/out/gl" || s.MediaRootDir != "/out/media" || s.CacheDir != "/out/cache" {
		t.Errorf("got gl=%q media=%q cache=%q", s.GamelistDir, s.MediaRootDir, s.CacheDir)
	}
}

func TestResolveScrapeSettings_CacheDirFallsBackToConfig(t *testing.T) {
	s, err := resolveScrapeSettings(validFlags(), testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.CacheDir != "/cfg/cache" {
		t.Errorf("CacheDir = %q, want /cfg/cache", s.CacheDir)
	}
}

func TestResolveScrapeSettings_ExpandsPlatformInFlagPaths(t *testing.T) {
	f := validFlags()
	f.romDir = "/roms/{platform}"
	f.gamelistDir = "/gl/{platform}"
	f.mediaRootDir = "/media/{platform}"
	f.cacheDir = "/cache/{platform}"
	s, err := resolveScrapeSettings(f, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.ROMDir != "/roms/nes" || s.GamelistDir != "/gl/nes" || s.MediaRootDir != "/media/nes" || s.CacheDir != "/cache/nes" {
		t.Errorf("got rom=%q gl=%q media=%q cache=%q", s.ROMDir, s.GamelistDir, s.MediaRootDir, s.CacheDir)
	}
}

func TestResolveScrapeSettings_DefaultedDirsAreExpanded(t *testing.T) {
	f := validFlags()
	f.romDir = "/roms/{platform}"
	s, err := resolveScrapeSettings(f, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.GamelistDir != "/roms/nes" || s.MediaRootDir != "/roms/nes" {
		t.Errorf("got gl=%q media=%q", s.GamelistDir, s.MediaRootDir)
	}
}

func TestResolveScrapeSettings_ExpandsPlatformInConfigCacheDir(t *testing.T) {
	cfg := testConfig()
	cfg.Output.CacheDir = "/home/u/.scrapegoat/{platform}"
	s, err := resolveScrapeSettings(validFlags(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if s.CacheDir != "/home/u/.scrapegoat/nes" {
		t.Errorf("CacheDir = %q, want /home/u/.scrapegoat/nes", s.CacheDir)
	}
	if cfg.Output.CacheDir != "/home/u/.scrapegoat/{platform}" {
		t.Errorf("config was mutated: %q", cfg.Output.CacheDir)
	}
}

func TestResolveScrapeSettings_AutoDelete(t *testing.T) {
	cases := []struct {
		value string
		want  unmatchedDisposition
	}{
		{"true", dispositionDelete},
		{"1", dispositionDelete},
		{"false", dispositionKeep},
		{"0", dispositionKeep},
		{"", dispositionAsk},
	}
	for _, c := range cases {
		f := validFlags()
		f.autoDelete = c.value
		s, err := resolveScrapeSettings(f, testConfig())
		if err != nil {
			t.Errorf("--auto-delete=%q: unexpected error %v", c.value, err)
			continue
		}
		if s.UnmatchedDisposition != c.want {
			t.Errorf("--auto-delete=%q: disposition = %v, want %v", c.value, s.UnmatchedDisposition, c.want)
		}
	}
}

func TestResolveScrapeSettings_AutoDeleteInvalid(t *testing.T) {
	f := validFlags()
	f.autoDelete = "yes"
	_, err := resolveScrapeSettings(f, testConfig())
	if err == nil || !strings.Contains(err.Error(), "--auto-delete") {
		t.Fatalf("err = %v, want error naming --auto-delete", err)
	}
}

func TestResolveScrapeSettings_Artwork(t *testing.T) {
	cases := []struct {
		value     string
		configVal bool
		want      bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"false", true, false},
		{"0", true, false},
		{"", true, true},
		{"", false, false},
	}
	for _, c := range cases {
		cfg := testConfig()
		cfg.Artwork.Enabled = c.configVal
		f := validFlags()
		f.artwork = c.value
		s, err := resolveScrapeSettings(f, cfg)
		if err != nil {
			t.Errorf("--artwork=%q: unexpected error %v", c.value, err)
			continue
		}
		if s.ArtworkEnabled != c.want {
			t.Errorf("--artwork=%q config=%v: ArtworkEnabled = %v, want %v", c.value, c.configVal, s.ArtworkEnabled, c.want)
		}
		if cfg.Artwork.Enabled != c.configVal {
			t.Errorf("--artwork=%q: config was mutated", c.value)
		}
	}
}

func TestResolveScrapeSettings_ArtworkInvalid(t *testing.T) {
	f := validFlags()
	f.artwork = "on"
	_, err := resolveScrapeSettings(f, testConfig())
	if err == nil || !strings.Contains(err.Error(), "--artwork") {
		t.Fatalf("err = %v, want error naming --artwork", err)
	}
}

func TestResolveScrapeSettings_CacheHoursMustBePositive(t *testing.T) {
	for _, h := range []int{0, -1} {
		f := validFlags()
		f.cacheHours = h
		_, err := resolveScrapeSettings(f, testConfig())
		if err == nil || !strings.Contains(err.Error(), "--cache-hours") {
			t.Errorf("--cache-hours=%d: err = %v, want error naming --cache-hours", h, err)
		}
	}
}

func TestResolveScrapeSettings_CopiesScalars(t *testing.T) {
	f := validFlags()
	f.workers = 7
	f.dryRun = true
	f.forceRedownload = true
	f.cacheHours = 3
	s, err := resolveScrapeSettings(f, testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if s.PlatformName != "nes" || s.Workers != 7 || !s.DryRun || !s.ForceRedownload || s.CacheHours != 3 {
		t.Errorf("got %+v", s)
	}
}
