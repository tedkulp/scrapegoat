package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tedkulp/scrapegoat/pkg/config"
)

// scrapeFlags holds the raw values of the scrape command's flags, exactly as
// Cobra bound them. Only flag registration and resolveScrapeSettings read it.
type scrapeFlags struct {
	platform        string
	romDir          string
	gamelistDir     string
	mediaRootDir    string
	cacheDir        string
	workers         int
	dryRun          bool
	autoDelete      string // a strconv.ParseBool value, or "" to ask
	artwork         string // a strconv.ParseBool value, or "" to use the config
	cacheHours      int
	forceRedownload bool
}

// scrapeSettings are the effective settings of one scrape run, resolved once
// from the flags and the loaded config. Nothing changes them afterwards.
type scrapeSettings struct {
	PlatformName         string
	ROMDir               string
	GamelistDir          string
	MediaRootDir         string
	CacheDir             string
	Workers              int
	DryRun               bool
	ForceRedownload      bool
	ArtworkEnabled       bool
	UnmatchedDisposition unmatchedDisposition
	CacheHours           int
}

// resolveScrapeSettings turns the scrape flags and the loaded config into the
// run's settings. Defaults are applied first (gamelist dir and media root
// fall back to the ROM dir, the cache dir to the config's cache_dir), then
// {platform} is expanded, so the placeholder also works in cache_dir from
// the config file. It neither reads nor changes anything else.
func resolveScrapeSettings(flags scrapeFlags, cfg *config.Config) (scrapeSettings, error) {
	disposition, err := parseDisposition(flags.autoDelete)
	if err != nil {
		return scrapeSettings{}, err
	}

	artworkEnabled := cfg.Artwork.Enabled
	if v, set, err := parseOptionalBool("--artwork", flags.artwork); err != nil {
		return scrapeSettings{}, err
	} else if set {
		artworkEnabled = v
	}

	if flags.cacheHours <= 0 {
		return scrapeSettings{}, fmt.Errorf("--cache-hours must be greater than 0, got %d", flags.cacheHours)
	}

	gamelistDir := flags.gamelistDir
	if gamelistDir == "" {
		gamelistDir = flags.romDir
	}
	mediaRootDir := flags.mediaRootDir
	if mediaRootDir == "" {
		mediaRootDir = flags.romDir
	}
	cacheDir := flags.cacheDir
	if cacheDir == "" {
		cacheDir = cfg.Output.CacheDir
	}

	p := flags.platform
	return scrapeSettings{
		PlatformName:         p,
		ROMDir:               expandPathTemplate(flags.romDir, p),
		GamelistDir:          expandPathTemplate(gamelistDir, p),
		MediaRootDir:         expandPathTemplate(mediaRootDir, p),
		CacheDir:             expandPathTemplate(cacheDir, p),
		Workers:              flags.workers,
		DryRun:               flags.dryRun,
		ForceRedownload:      flags.forceRedownload,
		ArtworkEnabled:       artworkEnabled,
		UnmatchedDisposition: disposition,
		CacheHours:           flags.cacheHours,
	}, nil
}

// parseDisposition maps --auto-delete to an unmatched ROM disposition:
// true deletes, false keeps, unset asks.
func parseDisposition(value string) (unmatchedDisposition, error) {
	v, set, err := parseOptionalBool("--auto-delete", value)
	switch {
	case err != nil:
		return dispositionAsk, err
	case !set:
		return dispositionAsk, nil
	case v:
		return dispositionDelete, nil
	default:
		return dispositionKeep, nil
	}
}

// parseOptionalBool parses a tri-state flag: "" is unset, anything else must
// be a value strconv.ParseBool accepts (case and surrounding space ignored).
func parseOptionalBool(flag, value string) (v bool, set bool, err error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false, false, nil
	}
	v, err = strconv.ParseBool(value)
	if err != nil {
		return false, false, fmt.Errorf("invalid value %q for %s: want true, false, 1, 0 or unset", value, flag)
	}
	return v, true, nil
}

// expandPathTemplate replaces template variables in a path string
// Currently supports: {platform}
func expandPathTemplate(path string, platformName string) string {
	return strings.ReplaceAll(path, "{platform}", platformName)
}
