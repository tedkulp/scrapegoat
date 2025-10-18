package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/spf13/viper"
	"github.com/tedkulp/scrapegoat/internal/artwork"
	"github.com/tedkulp/scrapegoat/internal/scraper"
)

// Config represents the application configuration
type Config struct {
	ScreenScraper  ScreenScraperConfig `mapstructure:"screenscraper"`
	Platforms      map[string]Platform `mapstructure:"platforms"`
	Output         OutputConfig        `mapstructure:"output"`
	Artwork        artwork.Config      `mapstructure:"artwork"`
	platformsCache *PlatformsCache
	cacheExpHours  int // Cache expiration in hours for platforms and ROM metadata (default: 12)
}

// ScreenScraperConfig holds API authentication credentials
type ScreenScraperConfig struct {
	DevID        string `mapstructure:"dev_id"`
	DevPassword  string `mapstructure:"dev_password"`
	UserID       string `mapstructure:"user_id"`
	UserPassword string `mapstructure:"user_password"`
	SoftwareName string `mapstructure:"software_name"`
}

// Platform defines a gaming platform/system
type Platform struct {
	ID         int      `mapstructure:"id"`
	Name       string   `mapstructure:"name"`
	Extensions []string `mapstructure:"extensions"`
}

// OutputConfig defines output directory settings
type OutputConfig struct {
	GameListFile string `mapstructure:"gamelist_file"`
	MediaDir     string `mapstructure:"media_dir"`
	TempDir      string `mapstructure:"temp_dir"` // Deprecated: use CacheDir instead
	CacheDir     string `mapstructure:"cache_dir"`
}

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Look for config in home directory or current directory
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "scrapegoat"))
		}
		v.AddConfigPath(".")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
	}

	// Environment variable support
	v.SetEnvPrefix("ESCRAPE")
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("screenscraper.software_name", "scrapegoat/1.0")
	v.SetDefault("output.gamelist_file", "gamelist.xml")
	v.SetDefault("output.media_dir", "media")
	v.SetDefault("output.temp_dir", "/tmp/scrapegoat") // Deprecated

	// Default cache dir to ~/.scrapegoat
	home, err := os.UserHomeDir()
	if err == nil {
		v.SetDefault("output.cache_dir", filepath.Join(home, ".scrapegoat"))
		v.SetDefault("artwork.resources_dir", filepath.Join(home, ".scrapegoat", "resources"))
	} else {
		v.SetDefault("output.cache_dir", "/tmp/scrapegoat")
		v.SetDefault("artwork.resources_dir", "/tmp/scrapegoat/resources")
	}

	// Artwork defaults
	v.SetDefault("artwork.enabled", false)
	v.SetDefault("artwork.output_dir", "artwork")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if config.ScreenScraper.DevID == "" {
		return nil, fmt.Errorf("screenscraper.dev_id is required")
	}
	if config.ScreenScraper.DevPassword == "" {
		return nil, fmt.Errorf("screenscraper.dev_password is required")
	}
	if config.ScreenScraper.UserID == "" {
		return nil, fmt.Errorf("screenscraper.user_id is required")
	}
	if config.ScreenScraper.UserPassword == "" {
		return nil, fmt.Errorf("screenscraper.user_password is required")
	}

	return &config, nil
}

// SetCacheExpiration sets the cache expiration time in hours for both platforms and ROM metadata
func (c *Config) SetCacheExpiration(hours int) {
	c.cacheExpHours = hours
}

// GetCacheExpiration returns the cache expiration time in hours
func (c *Config) GetCacheExpiration() int {
	if c.cacheExpHours <= 0 {
		return 12 // default
	}
	return c.cacheExpHours
}

// GetPlatform returns platform configuration by name
// If not found in config.yaml, attempts to fetch from ScreenScraper API
func (c *Config) GetPlatform(name string) (Platform, error) {
	// First, try static platforms from config.yaml
	platform, ok := c.Platforms[name]
	if ok {
		return platform, nil
	}

	// If not in config, fetch from API cache
	if c.platformsCache == nil {
		if err := c.loadPlatformsFromAPI(); err != nil {
			return Platform{}, fmt.Errorf("platform %s not found and failed to fetch from API: %w", name, err)
		}
	}

	// Try to find platform in cache by name (case-insensitive)
	platform, ok = c.platformsCache.FindByName(name)
	if !ok {
		return Platform{}, fmt.Errorf("platform %s not found in configuration or API", name)
	}

	return platform, nil
}

// PlatformsCache stores cached platform data from ScreenScraper API
type PlatformsCache struct {
	FetchedAt time.Time           `json:"fetched_at"`
	Platforms map[string]Platform `json:"platforms"` // Key is platform name (lowercase)
}

// FindByName searches for a platform by name (case-insensitive)
func (pc *PlatformsCache) FindByName(name string) (Platform, bool) {
	platform, ok := pc.Platforms[strings.ToLower(name)]
	return platform, ok
}

// IsExpired checks if the cache is older than the specified duration
func (pc *PlatformsCache) IsExpired(expirationHours int) bool {
	return time.Since(pc.FetchedAt) > time.Duration(expirationHours)*time.Hour
}

// loadPlatformsFromAPI fetches platforms from ScreenScraper and caches them
func (c *Config) loadPlatformsFromAPI() error {
	cacheFile := filepath.Join(c.Output.CacheDir, "platforms.json")

	// Try to load from cache file first
	if cache, err := loadPlatformsCache(cacheFile); err == nil && !cache.IsExpired(c.GetCacheExpiration()) {
		// Apply platform aliases even when loading from cache
		applyPlatformAliases(cache.Platforms)
		c.platformsCache = cache
		return nil
	}

	// Cache is expired or doesn't exist, fetch from API
	client := scraper.NewClient(
		c.ScreenScraper.DevID,
		c.ScreenScraper.DevPassword,
		c.ScreenScraper.UserID,
		c.ScreenScraper.UserPassword,
		c.ScreenScraper.SoftwareName,
	)

	systems, err := client.GetSystemsList()
	if err != nil {
		return fmt.Errorf("failed to fetch systems list from API: %w", err)
	}

	// Convert systems to platforms map
	platforms := make(map[string]Platform)
	preferredRegions := []string{"us", "wor", "eu", "jp"}

	for _, system := range systems {
		name := system.GetPreferredName(preferredRegions)
		if name == "" {
			continue
		}

		platform := Platform{
			ID:         system.ID,
			Name:       name,
			Extensions: system.Extensions,
		}

		// Create proper URL-safe slugs from the name
		// This handles special characters like ², é, etc.
		// Remove dashes to make slugs more compact (e.g., "n64dd" instead of "n64-dd")
		fullSlug := strings.ReplaceAll(slug.Make(name), "-", "")

		// Store under full slug
		platforms[fullSlug] = platform

		// Also store under each individual slug part (if comma-separated)
		// This allows users to use short names like "nes" instead of the full slug
		if strings.Contains(name, ",") {
			nameParts := strings.Split(name, ",")
			for _, namePart := range nameParts {
				namePart = strings.TrimSpace(namePart)
				if namePart != "" {
					partSlug := strings.ReplaceAll(slug.Make(namePart), "-", "")
					// Only store if not already taken by another platform
					if _, exists := platforms[partSlug]; !exists {
						platforms[partSlug] = platform
					}
				}
			}
		}
	}

	// Apply platform aliases (allows custom slug names)
	applyPlatformAliases(platforms)

	// Create and save cache
	cache := &PlatformsCache{
		FetchedAt: time.Now(),
		Platforms: platforms,
	}

	if err := savePlatformsCache(cache, cacheFile); err != nil {
		// Log warning but don't fail - we have the data
		fmt.Fprintf(os.Stderr, "Warning: failed to save platforms cache: %v\n", err)
	}

	c.platformsCache = cache
	return nil
}

// loadPlatformsCache loads the cached platforms from disk
func loadPlatformsCache(cacheFile string) (*PlatformsCache, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache PlatformsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	return &cache, nil
}

// savePlatformsCache saves the platforms cache to disk
func savePlatformsCache(cache *PlatformsCache, cacheFile string) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}
