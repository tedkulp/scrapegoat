package main

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tedkulp/scrapegoat/internal/platform"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

type countingSource struct {
	systems []screenscraper.System
	calls   int
}

func (s *countingSource) GetSystemsList() ([]screenscraper.System, error) {
	s.calls++
	return s.systems, nil
}

func testCatalog(src platform.SystemsSource, dir string) *platform.Catalog {
	return platform.New(platform.Options{
		Source: src,
		Configured: map[string]platform.Platform{
			"mybox": {ID: 900, Name: "My Box,Box", Extensions: []string{".box"}},
		},
		CacheDir: dir,
		Expiry:   time.Duration(defaultCacheHours) * time.Hour,
	})
}

func sys(id int, name string) screenscraper.System {
	return screenscraper.System{ID: id, Names: []screenscraper.Name{{Region: "us", Text: name}}}
}

func TestListPlatforms_EverySlugResolvesToItsRow(t *testing.T) {
	src := &countingSource{systems: []screenscraper.System{
		sys(3, "Nintendo Entertainment System,NES,Famicom"),
		sys(26, "Atari2600,A2600,2600"),
		sys(42, "Atari ST,Atari STE,520 ST"),
		sys(80, "NES"), // loses "nes" to ID 3 and has no other slug
	}}
	catalog := testCatalog(src, t.TempDir())

	var out bytes.Buffer
	if err := listPlatforms(&out, catalog); err != nil {
		t.Fatal(err)
	}

	rows := 0
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		rows++
		if fields[1] == "-" {
			continue
		}
		p, err := catalog.Lookup(fields[1])
		if err != nil {
			t.Errorf("listed slug %q: %v", fields[1], err)
			continue
		}
		if p.ID != id {
			t.Errorf("listed slug %q resolves to %d, row is %d", fields[1], p.ID, id)
		}
	}
	if rows != 5 {
		t.Fatalf("listed %d rows, want 5:\n%s", rows, out.String())
	}
	for _, want := range []string{"atari2600", "My Box (config)", "Atari ST "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestListPlatforms_FreshCacheMakesNoRequest(t *testing.T) {
	dir := t.TempDir()
	first := &countingSource{systems: []screenscraper.System{sys(3, "NES")}}
	if err := listPlatforms(&bytes.Buffer{}, testCatalog(first, dir)); err != nil {
		t.Fatal(err)
	}
	if first.calls != 1 {
		t.Fatalf("first listing made %d requests, want 1", first.calls)
	}

	second := &countingSource{}
	var out bytes.Buffer
	if err := listPlatforms(&out, testCatalog(second, dir)); err != nil {
		t.Fatal(err)
	}
	if second.calls != 0 {
		t.Fatalf("listing with a fresh platforms.json made %d requests", second.calls)
	}
	if !strings.Contains(out.String(), "nes") {
		t.Fatalf("cached platform missing:\n%s", out.String())
	}
}

func TestListPlatformsCacheDir_DropsThePlatformElement(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/home/u/.scrapegoat", "/home/u/.scrapegoat"},
		{"/home/u/.scrapegoat/{platform}", "/home/u/.scrapegoat"},
		{"/home/u/.scrapegoat/{platform}/", "/home/u/.scrapegoat"},
		{"/home/u/.scrapegoat/cache-{platform}/x", "/home/u/.scrapegoat"},
		{"/home/u/{platform}/.scrapegoat", "/home/u"},
		{"{platform}", "."},
	}
	for _, tt := range tests {
		if got := listPlatformsCacheDir(filepath.FromSlash(tt.in)); got != filepath.FromSlash(tt.want) {
			t.Errorf("listPlatformsCacheDir(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
