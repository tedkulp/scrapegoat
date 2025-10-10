package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	ScreenScraper ScreenScraperConfig `mapstructure:"screenscraper"`
	Platforms     map[string]Platform `mapstructure:"platforms"`
	Output        OutputConfig        `mapstructure:"output"`
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
	} else {
		v.SetDefault("output.cache_dir", "/tmp/scrapegoat")
	}

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

// GetPlatform returns platform configuration by name
func (c *Config) GetPlatform(name string) (Platform, error) {
	platform, ok := c.Platforms[name]
	if !ok {
		return Platform{}, fmt.Errorf("platform %s not found in configuration", name)
	}
	return platform, nil
}
