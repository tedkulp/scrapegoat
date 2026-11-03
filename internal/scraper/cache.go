package scraper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GameCache handles caching of game information
type GameCache struct {
	cacheDir        string
	expirationHours int
	now             func() time.Time
}

// NewGameCache creates a new game cache that reads the current time from now.
func NewGameCache(cacheDir string, expirationHours int, now func() time.Time) *GameCache {
	if expirationHours <= 0 {
		expirationHours = 12
	}
	return &GameCache{
		cacheDir:        cacheDir,
		expirationHours: expirationHours,
		now:             now,
	}
}

// getCacheKey generates a unique cache key for a ROM based on its hashes
func (gc *GameCache) getCacheKey(hashes *ROMHashes) string {
	// Use MD5 hash as the primary key (it's the most reliable identifier)
	return hashes.MD5
}

// getCacheFilePath returns the full path to the cache file for a ROM
func (gc *GameCache) getCacheFilePath(cacheKey string) (string, error) {
	if len(cacheKey) < 2 {
		return "", fmt.Errorf("cache key %q is too short", cacheKey)
	}
	// Use first 2 characters of hash for subdirectory (like git does)
	// This prevents too many files in a single directory
	subdir := cacheKey[:2]
	return filepath.Join(gc.cacheDir, "games", subdir, cacheKey+".json"), nil
}

// CacheResult represents a cache lookup result
type CacheResult struct {
	Game      *Game
	Found     bool // True if cache entry exists (and, for Get, is not expired)
	NotFound  bool // True if ROM was previously looked up and not found
	IsNonGame bool // True if ROM was previously looked up and is a non-game
}

// Get retrieves cached game info if available and not expired
// Returns a CacheResult with information about what was found in cache
func (gc *GameCache) Get(hashes *ROMHashes) CacheResult {
	return gc.get(hashes, true)
}

// GetAny retrieves cached game info whatever its age. It is for runs that
// ScreenScraper has cut off, where an old answer beats none.
func (gc *GameCache) GetAny(hashes *ROMHashes) CacheResult {
	return gc.get(hashes, false)
}

func (gc *GameCache) get(hashes *ROMHashes, checkExpiry bool) CacheResult {
	cacheFile, err := gc.getCacheFilePath(gc.getCacheKey(hashes))
	if err != nil {
		return CacheResult{Found: false}
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return CacheResult{Found: false}
	}

	var entry GameCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return CacheResult{Found: false}
	}

	if checkExpiry && entry.IsExpired(gc.now(), gc.expirationHours) {
		return CacheResult{Found: false}
	}

	// Return cached result (could be positive or negative)
	return CacheResult{
		Game:      entry.Game,
		Found:     true,
		NotFound:  entry.NotFound,
		IsNonGame: entry.IsNonGame,
	}
}

// Set saves game info to cache
func (gc *GameCache) Set(hashes *ROMHashes, game *Game) error {
	return gc.setCacheEntry(hashes, GameCacheEntry{
		FetchedAt: gc.now(),
		Game:      game,
		NotFound:  false,
		IsNonGame: false,
	})
}

// SetNotFound caches a "not found" result for a ROM
func (gc *GameCache) SetNotFound(hashes *ROMHashes) error {
	return gc.setCacheEntry(hashes, GameCacheEntry{
		FetchedAt: gc.now(),
		Game:      nil,
		NotFound:  true,
		IsNonGame: false,
	})
}

// SetNonGame caches a "non-game" result for a ROM
func (gc *GameCache) SetNonGame(hashes *ROMHashes, game *Game) error {
	return gc.setCacheEntry(hashes, GameCacheEntry{
		FetchedAt: gc.now(),
		Game:      game,
		NotFound:  false,
		IsNonGame: true,
	})
}

// setCacheEntry is the internal method that writes cache entries
func (gc *GameCache) setCacheEntry(hashes *ROMHashes, entry GameCacheEntry) error {
	cacheFile, err := gc.getCacheFilePath(gc.getCacheKey(hashes))
	if err != nil {
		return err
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}
