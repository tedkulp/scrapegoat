package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tedkulp/scrapegoat/internal/artwork"
	_ "github.com/tedkulp/scrapegoat/internal/artwork/effects" // Register effects
	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/platform"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
	"github.com/tedkulp/scrapegoat/pkg/config"
)

var (
	version = "dev"
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "scrapegoat",
	Short: "ROM scraper for EmulationStation using ScreenScraper.fr API",
	Long: `Scrapegoat is a CLI tool that scrapes ROM files using the ScreenScraper.fr API.
It downloads game metadata and media files, then generates a gamelist.xml file
compatible with EmulationStation.`,
}

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape ROM files and generate gamelist.xml",
	Long: `Scrape ROM files using ScreenScraper.fr API to fetch metadata and media files.
Generates a gamelist.xml file compatible with EmulationStation.`,
	Run: runScraper,
}

var listPlatformsCmd = &cobra.Command{
	Use:   "list-platforms",
	Short: "List every platform and the slug that picks it",
	Long: `Lists every platform in the platform catalog: ScreenScraper's platforms and
any defined in config.yaml (marked "(config)"), each with the slug that picks it.
ScreenScraper's list is cached in <cache_dir>/platforms.json and refetched after 12 hours.`,
	Run: runListPlatforms,
}

var userInfoCmd = &cobra.Command{
	Use:   "user-info",
	Short: "Display user account and API quota information",
	Long:  `Fetches and displays your ScreenScraper account information including API request limits and usage.`,
	Run:   runUserInfo,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version of scrapegoat.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("scrapegoat version %s\n", version)
	},
}

// defaultCacheHours is how long platforms.json and game lookups stay fresh
// unless --cache-hours says otherwise; list-platforms always uses it.
const defaultCacheHours = 12

// scrapeFlagValues is what Cobra binds the scrape flags into.
var scrapeFlagValues scrapeFlags

func init() {
	// Root persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml or ~/.config/scrapegoat/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Scrape command flags
	scrapeCmd.Flags().StringVarP(&scrapeFlagValues.platform, "platform", "p", "", "platform/system name (required)")
	scrapeCmd.Flags().StringVarP(&scrapeFlagValues.romDir, "rom-dir", "r", "", "ROM directory path (required)")
	scrapeCmd.Flags().StringVarP(&scrapeFlagValues.gamelistDir, "gamelist-dir", "o", "", "directory for gamelist.xml output (default: same as rom-dir)")
	scrapeCmd.Flags().StringVarP(&scrapeFlagValues.mediaRootDir, "media-root-dir", "m", "", "root directory for media files (default: same as rom-dir)")
	scrapeCmd.Flags().StringVar(&scrapeFlagValues.cacheDir, "cache-dir", "", "cache directory for the platform catalog, game lookups and media (default: output.cache_dir, then ~/.scrapegoat)")
	scrapeCmd.Flags().IntVarP(&scrapeFlagValues.workers, "workers", "w", 4, "number of concurrent download workers")
	scrapeCmd.Flags().BoolVar(&scrapeFlagValues.dryRun, "dry-run", false, "scan and query API but don't download or write files")
	scrapeCmd.Flags().StringVar(&scrapeFlagValues.autoDelete, "auto-delete", "", "delete unmatched ROMs (not found, non-game, or failed): true/1=delete, false/0=keep, unset=prompt (default: prompt)")
	scrapeCmd.Flags().StringVar(&scrapeFlagValues.artwork, "artwork", "", "enable/disable artwork generation: true/1=enable, false/0=disable, unset=use config (default: use config)")
	scrapeCmd.Flags().IntVar(&scrapeFlagValues.cacheHours, "cache-hours", defaultCacheHours, "hours before cache expires for both platforms and ROM metadata, must be greater than 0")
	scrapeCmd.Flags().BoolVar(&scrapeFlagValues.forceRedownload, "force-redownload", false, "re-download every media file from ScreenScraper, skipping both the media root and the media cache (counts against the daily quota)")

	scrapeCmd.MarkFlagRequired("platform")
	scrapeCmd.MarkFlagRequired("rom-dir")

	// Add subcommands
	rootCmd.AddCommand(scrapeCmd)
	rootCmd.AddCommand(listPlatformsCmd)
	rootCmd.AddCommand(userInfoCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runListPlatforms(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	catalog := newPlatformCatalog(cfg, listPlatformsCacheDir(cfg.Output.CacheDir), defaultCacheHours)
	if err := listPlatforms(os.Stdout, catalog); err != nil {
		log.Fatalf("Failed to list platforms: %v", err)
	}

	logInfo("\nTo use a platform, specify the slug with the --platform flag.")
	logInfo("Example: scrapegoat scrape --platform nes --rom-dir /path/to/roms")
}

// listPlatformsCacheDir is the cache dir list-platforms uses for the config's
// cache_dir. list-platforms has no --platform to expand {platform} with, so
// the path is cut before the first element containing it: the parent of the
// per-platform caches, never a directory literally named {platform}.
func listPlatformsCacheDir(cacheDir string) string {
	dir := cacheDir
	for strings.Contains(dir, "{platform}") {
		dir = filepath.Dir(dir)
	}
	return dir
}

// listPlatforms writes the platform catalog as a table, one platform per row
// with the slug that picks it. Platforms from config.yaml are marked.
func listPlatforms(w io.Writer, catalog *platform.Catalog) error {
	entries, err := catalog.List()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Found %d platforms:\n\n", len(entries))
	fmt.Fprintf(w, "%-5s %-25s %-30s %s\n", "ID", "Slug", "Platform Name", "Extensions")
	fmt.Fprintf(w, "%-5s %-25s %-30s %s\n", "-----", "-------------------------", "------------------------------", "----------")
	for _, e := range entries {
		slug := e.Slug
		if slug == "" {
			slug = "-"
		}
		name := e.Platform.Name
		if e.FromConfig {
			name += " (config)"
		}
		fmt.Fprintf(w, "%-5d %-25s %-30s %s\n", e.Platform.ID, slug, name, strings.Join(e.Platform.Extensions, ", "))
	}
	return nil
}

// newPlatformCatalog builds the platform catalog from config.yaml's
// platforms and ScreenScraper, cached in cacheDir for cacheHours.
func newPlatformCatalog(cfg *config.Config, cacheDir string, cacheHours int) *platform.Catalog {
	configured := make(map[string]platform.Platform, len(cfg.Platforms))
	for slug, p := range cfg.Platforms {
		configured[slug] = platform.Platform{ID: p.ID, Name: p.Name, Extensions: p.Extensions}
	}
	return platform.New(platform.Options{
		Source:     screenscraper.NewClient(screenScraperCredentials(cfg.ScreenScraper), screenscraper.WithDebug(verbose)),
		Configured: configured,
		CacheDir:   cacheDir,
		Expiry:     time.Duration(cacheHours) * time.Hour,
	})
}

// screenScraperCredentials converts the config's credentials for the client.
func screenScraperCredentials(c config.ScreenScraperConfig) screenscraper.Credentials {
	return screenscraper.Credentials{
		DevID:        c.DevID,
		DevPassword:  c.DevPassword,
		UserID:       c.UserID,
		UserPassword: c.UserPassword,
		SoftwareName: c.SoftwareName,
	}
}

func runScraper(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	settings, err := resolveScrapeSettings(scrapeFlagValues, cfg)
	if err != nil {
		log.Fatalf("Invalid scrape settings: %v", err)
	}
	platformName := settings.PlatformName
	romDir := settings.ROMDir
	gamelistDir := settings.GamelistDir
	mediaRootDir := settings.MediaRootDir
	cacheDir := settings.CacheDir
	dryRun := settings.DryRun

	// Get platform configuration; platforms.json lives in the resolved
	// cache dir, like every other cache.
	platform, err := newPlatformCatalog(cfg, cacheDir, settings.CacheHours).Lookup(platformName)
	if err != nil {
		log.Fatalf("Failed to get platform configuration: %v", err)
	}

	logInfo("Scrapegoat - ROM Scraper")
	logInfo("Platform: %s (ID: %d)", platform.Name, platform.ID)
	logInfo("ROM Directory: %s", romDir)
	logInfo("Gamelist Directory: %s", gamelistDir)
	logInfo("Media Root Directory: %s", mediaRootDir)
	logInfo("Cache Directory: %s", cacheDir)
	logInfo("")

	// Step 1: Scan for ROM files
	logInfo("Scanning for ROM files...")
	if len(platform.Extensions) == 0 {
		logInfo("  No file extensions specified - scanning all files")
	} else {
		logVerbose("  Extensions: %s", strings.Join(platform.Extensions, ", "))
	}
	romScanner := scanner.NewScanner(platform.Extensions)
	roms, err := romScanner.Scan(romDir)
	if err != nil {
		log.Fatalf("Failed to scan ROM directory: %v", err)
	}
	logInfo("Found %d ROM files", len(roms))

	if len(roms) == 0 {
		logInfo("No ROM files found. Exiting.")
		return
	}

	// Step 2: Initialize scraper client. One cutoff is shared by lookups and
	// media, so after the first cutoff status neither sends a request.
	cut := cutoff.New()
	scraperClient := screenscraper.NewClient(screenScraperCredentials(cfg.ScreenScraper),
		screenscraper.WithCache(cacheDir, settings.CacheHours),
		screenscraper.WithDebug(verbose),
		screenscraper.WithCutoff(cut),
	)

	mediaOpts := []media.Option{media.WithCutoff(cut)}
	if verbose {
		mediaOpts = append(mediaOpts, media.WithDebugOutput(os.Stderr))
	}
	placer := media.New(cacheDir, platformName, mediaRootDir, settings.Workers, mediaOpts...)

	// Step 3.5: Initialize artwork compositor if enabled
	var compositor *artwork.Compositor
	if settings.ArtworkEnabled && !dryRun {
		compositor = artwork.NewCompositor(cfg.Artwork, cacheDir)
		compositor.SetVerbose(verbose)
		logInfo("Artwork generation: Enabled")
	}

	// Step 4: Initialize metadata generator (not used in a dry run)
	// Note: mediaRootDir is used directly, no longer needs cfg.Output.MediaDir subdirectory
	gamelistPath := filepath.Join(gamelistDir, cfg.Output.GameListFile)
	var gen *metadata.Generator
	if !dryRun {
		gen = metadata.NewGenerator(gamelistDir, mediaRootDir)
	}

	// Step 5: Scrape each ROM
	loop := &romLoop{
		scraper: scrape.New(scraperClient, placer, compositor, platform.ID,
			media.Mode{DryRun: dryRun, Force: settings.ForceRedownload}),
		cutoff:       cut,
		gen:          gen,
		gamelistPath: gamelistPath,
		quota:        scraperClient.Quota,
		report:       printReport,
	}

	logInfo("\nProcessing ROM files...")
	tally := loop.run(roms)

	// Step 6: Write gamelist.xml. After a cutoff, deferred ROMs' existing
	// entries were carried over.
	if !dryRun {
		logInfo("\nWriting gamelist.xml...")
		if err := gen.WriteToFile(gamelistPath); err != nil {
			log.Fatalf("Failed to write gamelist.xml: %v", err)
		}
		logInfo("Gamelist written to: %s", gamelistPath)
		logInfo("Media files location: %s", mediaRootDir)
	}

	// Apply the unmatched ROM disposition (keep, delete, or ask)
	if len(tally.unmatched) > 0 && !dryRun {
		handleUnmatchedROMs(settings.UnmatchedDisposition, tally.unmatched)
	}

	// Summary
	logInfo("\n=== Summary ===")
	logInfo("Total ROMs processed: %d", len(roms))
	for _, line := range tally.summaryLines() {
		logInfo("%s", line)
	}

	// API call statistics
	if totalAPICalls := scraperClient.RequestCount(); totalAPICalls > 0 && !dryRun {
		logInfo("\n=== API Call Statistics ===")
		logInfo("Total API calls: %d", totalAPICalls)
		if tally.scraped > 0 {
			avgPerROM := float64(totalAPICalls) / float64(tally.scraped)
			logInfo("Average API calls per ROM: %.2f", avgPerROM)
		}
		logInfo("(Run with --verbose to see detailed API call logs)")
	}

	if dryRun {
		logInfo("\nDRY RUN completed - no files were written")
	} else {
		logInfo("\nScraping complete!")
	}

	// Show cache info if verbose
	if verbose {
		logInfo("\n=== Cache Info ===")
		logInfo("Cache directory: %s/games", cacheDir)
		logInfo("%s", cacheDurationMessage(settings.CacheHours))
	}

	// A cutoff exits 1, after the gamelist is written.
	if code := exitCode(cut); code != 0 {
		os.Exit(code)
	}
}

// handleUnmatchedROMs lists the unmatched ROMs, applies the disposition with
// stdin as the prompt and os.Remove as the remover, and logs the outcome.
func handleUnmatchedROMs(d unmatchedDisposition, roms []scanner.ROMFile) {
	logInfo("\n=== Unmatched ROMs ===")
	logInfo("The following ROMs were not found, were marked non-game, or failed:")
	for i, rom := range roms {
		logInfo("  %d. %s", i+1, rom.Filename)
	}

	readLine := lineReader(os.Stdin)
	prompt := func() (string, error) {
		logInfo("\nWould you like to delete these ROM files? [y/N]: ")
		return readLine()
	}

	switch d {
	case dispositionDelete:
		logInfo("\nAuto-deleting unmatched ROM files (--auto-delete=true)...")
	case dispositionKeep:
		logInfo("\nKeeping unmatched ROM files (--auto-delete=false)")
	}

	res := disposeUnmatchedROMs(d, roms, prompt, os.Remove)

	if !res.Deleting {
		if d == dispositionAsk {
			logInfo("\nSkipping deletion. Unmatched ROM files were kept.")
		}
		return
	}
	if d == dispositionAsk {
		logInfo("\nDeleting unmatched ROM files...")
	}
	for _, rom := range res.Deleted {
		logInfo("  Deleted: %s", rom.Filename)
	}
	for _, f := range res.Failed {
		logError("  Failed to delete %s: %v", f.ROM.Filename, f.Err)
	}
	logInfo("\nDeletion complete: %d deleted, %d errors", len(res.Deleted), len(res.Failed))
}

func cacheDurationMessage(hours int) string {
	unit := "hours"
	if hours == 1 {
		unit = "hour"
	}
	return fmt.Sprintf("Game data is cached for %d %s", hours, unit)
}

// mediaSummary describes a ROM's placed media, listing only non-zero counts.
func mediaSummary(r media.Result) string {
	var parts []string
	for _, c := range []struct {
		n     int
		label string
	}{
		{r.Existing, "existing"},
		{r.Downloaded, "downloaded"},
		{r.FromCache, "from cache"},
		{r.Failed, "failed"},
		{r.Deferred, "deferred (cutoff)"},
	} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.label))
		}
	}
	return strings.Join(parts, ", ")
}

func logInfo(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func logVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func quotaLines(userInfo *screenscraper.UserInfo) []string {
	return []string{
		fmt.Sprintf("Requests Today: %s", userInfo.RequestsToday),
		fmt.Sprintf("Max Requests Per Day: %s", userInfo.MaxRequestsPerDay),
		fmt.Sprintf("Max Requests Per Minute: %s", userInfo.MaxRequestsPerMinute),
		fmt.Sprintf("Failed Requests Today: %s / %s", userInfo.RequestsKOToday, userInfo.MaxRequestsKOPerDay),
		fmt.Sprintf("Max Threads: %s", userInfo.MaxThreads),
	}
}

func runUserInfo(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logInfo("Fetching user information from ScreenScraper.fr...")

	// Create scraper client
	client := screenscraper.NewClient(screenScraperCredentials(cfg.ScreenScraper))

	// Fetch user info
	userInfo, err := client.GetUserInfo()
	if err != nil {
		log.Fatalf("Failed to fetch user info: %v", err)
	}

	// Display user information
	logInfo("\n=== Account Information ===")
	logInfo("User ID: %s", userInfo.ID)
	logInfo("Level: %s", userInfo.Level)
	logInfo("Contribution: %s", userInfo.Contribution)
	logInfo("ROMs Associated: %s", userInfo.ROMsAssociated)
	logInfo("Uploads: %s", userInfo.Uploads)

	logInfo("\n=== API Quota ===")
	for _, line := range quotaLines(userInfo) {
		logInfo("%s", line)
	}

	if quota := client.Quota(); quota.Known {
		logInfo("\nRemaining Requests Today: %d", quota.Remaining())
	}
}
