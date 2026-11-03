package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

const (
	cacheFileName = "platforms.json"
	// cacheFormat marks the raw-systems format. A platforms.json without it
	// (the old slug map) counts as expired.
	cacheFormat = 2
)

// cacheFile is platforms.json: ScreenScraper's systems list as fetched.
// Slugs are worked out when it is loaded.
type cacheFile struct {
	Format    int            `json:"format"`
	FetchedAt time.Time      `json:"fetched_at"`
	Systems   []cachedSystem `json:"systems"`
}

type cachedSystem struct {
	ID         int                  `json:"id"`
	Names      []screenscraper.Name `json:"names"`
	Extensions []string             `json:"extensions"`
}

func (c *Catalog) cachePath() string {
	return filepath.Join(c.opts.CacheDir, cacheFileName)
}

// systems returns the systems list from a fresh platforms.json, or fetches
// it and rewrites that file.
func (c *Catalog) systems() ([]screenscraper.System, error) {
	if systems, ok := c.readCache(); ok {
		return systems, nil
	}
	if c.opts.Source == nil {
		return nil, fmt.Errorf("no ScreenScraper source")
	}
	systems, err := c.opts.Source.GetSystemsList()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch systems list from API: %w", err)
	}
	if err := c.writeCache(systems); err != nil {
		// The data is in hand; a cache that can't be written only costs a
		// fetch next run.
		fmt.Fprintf(os.Stderr, "Warning: failed to save platforms cache: %v\n", err)
	}
	return systems, nil
}

func (c *Catalog) readCache() ([]screenscraper.System, bool) {
	data, err := os.ReadFile(c.cachePath())
	if err != nil {
		return nil, false
	}
	var f cacheFile
	if err := json.Unmarshal(data, &f); err != nil || f.Format != cacheFormat {
		return nil, false
	}
	if c.opts.Clock.Now().Sub(f.FetchedAt) > c.opts.Expiry {
		return nil, false
	}
	systems := make([]screenscraper.System, len(f.Systems))
	for i, s := range f.Systems {
		systems[i] = screenscraper.System{ID: s.ID, Names: s.Names, Extensions: s.Extensions}
	}
	return systems, true
}

func (c *Catalog) writeCache(systems []screenscraper.System) error {
	f := cacheFile{
		Format:    cacheFormat,
		FetchedAt: c.opts.Clock.Now(),
		Systems:   make([]cachedSystem, len(systems)),
	}
	for i, s := range systems {
		f.Systems[i] = cachedSystem{ID: s.ID, Names: s.Names, Extensions: s.Extensions}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}
	if err := os.MkdirAll(c.opts.CacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.WriteFile(c.cachePath(), data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
}
