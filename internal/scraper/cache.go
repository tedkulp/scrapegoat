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
	cacheDir string
}

// NewGameCache creates a new game cache
func NewGameCache(cacheDir string) *GameCache {
	return &GameCache{
		cacheDir: cacheDir,
	}
}

// getCacheKey generates a unique cache key for a ROM based on its hashes
func (gc *GameCache) getCacheKey(hashes *ROMHashes) string {
	// Use MD5 hash as the primary key (it's the most reliable identifier)
	return hashes.MD5
}

// getCacheFilePath returns the full path to the cache file for a ROM
func (gc *GameCache) getCacheFilePath(cacheKey string) string {
	// Use first 2 characters of hash for subdirectory (like git does)
	// This prevents too many files in a single directory
	subdir := cacheKey[:2]
	return filepath.Join(gc.cacheDir, "games", subdir, cacheKey+".json")
}

// Get retrieves cached game info if available and not expired
func (gc *GameCache) Get(hashes *ROMHashes) (*Game, bool) {
	cacheKey := gc.getCacheKey(hashes)
	cacheFile := gc.getCacheFilePath(cacheKey)

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}

	var entry GameCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check if expired
	if entry.IsExpired() {
		return nil, false
	}

	return entry.Game, true
}

// Set saves game info to cache
func (gc *GameCache) Set(hashes *ROMHashes, game *Game) error {
	cacheKey := gc.getCacheKey(hashes)
	cacheFile := gc.getCacheFilePath(cacheKey)

	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	entry := GameCacheEntry{
		FetchedAt: time.Now(),
		Game:      game,
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

// GetCacheStats returns statistics about the cache
func (gc *GameCache) GetCacheStats() (total int, expired int, err error) {
	gamesDir := filepath.Join(gc.cacheDir, "games")

	err = filepath.Walk(gamesDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		total++

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // Skip unreadable files
		}

		var entry GameCacheEntry
		if json.Unmarshal(data, &entry) == nil && entry.IsExpired() {
			expired++
		}

		return nil
	})

	return total, expired, err
}
