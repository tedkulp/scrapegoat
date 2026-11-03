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
	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/platform"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scraper"
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

	catalog := newPlatformCatalog(cfg, cfg.Output.CacheDir, defaultCacheHours)
	if err := listPlatforms(os.Stdout, catalog); err != nil {
		log.Fatalf("Failed to list platforms: %v", err)
	}

	logInfo("\nTo use a platform, specify the slug with the --platform flag.")
	logInfo("Example: scrapegoat scrape --platform nes --rom-dir /path/to/roms")
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
		Source:     scraper.NewClient(screenScraperCredentials(cfg.ScreenScraper), scraper.WithDebug(verbose)),
		Configured: configured,
		CacheDir:   cacheDir,
		Expiry:     time.Duration(cacheHours) * time.Hour,
	})
}

// screenScraperCredentials converts the config's credentials for the client.
func screenScraperCredentials(c config.ScreenScraperConfig) scraper.Credentials {
	return scraper.Credentials{
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

	// Step 2: Initialize scraper client
	scraperClient := scraper.NewClient(screenScraperCredentials(cfg.ScreenScraper),
		scraper.WithCache(cacheDir, settings.CacheHours),
		scraper.WithDebug(verbose),
	)

	mediaOpts := []media.Option{}
	if verbose {
		mediaOpts = append(mediaOpts, media.WithDebugOutput(os.Stderr))
	}
	placer := media.New(cacheDir, platformName, mediaRootDir, settings.Workers, mediaOpts...)
	mediaMode := media.Mode{DryRun: dryRun, Force: settings.ForceRedownload}

	// Step 3.5: Initialize artwork compositor if enabled
	var compositor *artwork.Compositor
	if settings.ArtworkEnabled && !dryRun {
		compositor = artwork.NewCompositor(cfg.Artwork, cacheDir)
		compositor.SetVerbose(verbose)
		logInfo("Artwork generation: Enabled")
	}

	// Step 4: Initialize metadata generator
	// Note: mediaRootDir is used directly, no longer needs cfg.Output.MediaDir subdirectory
	gen := metadata.NewGenerator(gamelistDir, mediaRootDir)

	// Step 5: Process each ROM
	logInfo("\nProcessing ROM files...")
	successCount := 0
	unmatchedCount := 0
	var unmatchedROMs []scanner.ROMFile // not found, non-game, or failed

	for i, rom := range roms {
		logInfo("\n[%d/%d] Processing: %s", i+1, len(roms), rom.Filename)

		// Hash the ROM file
		logVerbose("  Computing hashes...")
		hashes, err := scraper.HashROMFile(rom.Path)
		if err != nil {
			logError("  Failed to hash ROM: %v", err)
			unmatchedCount++
			unmatchedROMs = append(unmatchedROMs, rom)
			continue
		}
		logVerbose("  CRC32: %s, MD5: %s", hashes.CRC32, hashes.MD5)

		// Query ScreenScraper API (or get from cache)
		logVerbose("  Querying ScreenScraper API...")
		result, err := scraperClient.Lookup(platform.ID, rom.Filename, hashes)
		if err != nil {
			logError("  Failed to get game info: %v", err)
			unmatchedCount++
			unmatchedROMs = append(unmatchedROMs, rom)
			continue
		}

		if result.Outcome == scraper.OutcomeNotFound {
			if result.Cached {
				logError("  Failed to get game info: game not found in ScreenScraper database (cached)")
			} else {
				logError("  Failed to get game info: game not found in ScreenScraper database")
			}
			unmatchedCount++
			unmatchedROMs = append(unmatchedROMs, rom)
			continue
		}

		game := result.Game
		gameName := game.GetPreferredName(scraper.DefaultRegions())
		if result.Cached {
			logInfo("  Found: %s (cached)", gameName)
		} else {
			logInfo("  Found: %s", gameName)
		}

		if result.Outcome == scraper.OutcomeNonGame {
			logInfo("  Skipping: Marked as non-game by ScreenScraper")
			unmatchedCount++
			unmatchedROMs = append(unmatchedROMs, rom)
			continue
		}

		mediaPlan := placer.Plan(game, rom.Filename, mediaMode)

		if dryRun {
			logInfo("  [DRY RUN] Would download %d media files (%d from cache, %d already exist)",
				mediaPlan.Count(media.SourceDownload), mediaPlan.Count(media.SourceCache), mediaPlan.Count(media.SourceExisting))
			successCount++
			continue
		}

		romName := strings.TrimSuffix(rom.Filename, filepath.Ext(rom.Filename))

		logVerbose("  Placing %d media files (%d already exist)...", len(mediaPlan.Items), mediaPlan.Count(media.SourceExisting))
		mediaResult := placer.Execute(mediaPlan)
		for _, mediaType := range media.Types() {
			if err, ok := mediaResult.Failures[mediaType]; ok {
				logError("    Failed to place %s: %v", mediaType, err)
			}
		}
		if summary := mediaSummary(mediaResult); summary != "" {
			logInfo("  Media: %s", summary)
		}

		// Generate artwork if enabled
		if compositor != nil && len(mediaResult.Placed) > 0 {
			logVerbose("  Generating artwork...")
			artworkResult := compositor.GenerateArtwork(romName, artwork.MediaFiles(mediaResult.Placed), mediaRootDir)
			for _, err := range artworkResult.Errors {
				logInfo("  Warning: %v", err)
			}
			logVerbose("  Generated %d artwork files", len(artworkResult.Paths))
		}

		// Add game to metadata
		if err := gen.AddGameFromScraper(rom.Path, game); err != nil {
			logError("  Failed to add game metadata: %v", err)
			unmatchedCount++
			unmatchedROMs = append(unmatchedROMs, rom)
			continue
		}

		successCount++

		if quota := scraperClient.Quota(); quota.Known {
			logInfo("  API Quota: %d requests remaining today", quota.Remaining())
		}
	}

	// Step 6: Write gamelist.xml
	if !dryRun {
		logInfo("\nWriting gamelist.xml...")
		gamelistPath := filepath.Join(gamelistDir, cfg.Output.GameListFile)
		if err := gen.WriteToFile(gamelistPath); err != nil {
			log.Fatalf("Failed to write gamelist.xml: %v", err)
		}
		logInfo("Gamelist written to: %s", gamelistPath)
		logInfo("Media files location: %s", mediaRootDir)
	}

	// Apply the unmatched ROM disposition (keep, delete, or ask)
	if len(unmatchedROMs) > 0 && !dryRun {
		handleUnmatchedROMs(settings.UnmatchedDisposition, unmatchedROMs)
	}

	// Summary
	logInfo("\n=== Summary ===")
	logInfo("Total ROMs processed: %d", len(roms))
	logInfo("Successful: %d", successCount)
	logInfo("Unmatched: %d", unmatchedCount)

	// API call statistics
	if totalAPICalls := scraperClient.RequestCount(); totalAPICalls > 0 && !dryRun {
		logInfo("\n=== API Call Statistics ===")
		logInfo("Total API calls: %d", totalAPICalls)
		if successCount > 0 {
			avgPerROM := float64(totalAPICalls) / float64(successCount)
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

func quotaLines(userInfo *scraper.UserInfo) []string {
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
	client := scraper.NewClient(screenScraperCredentials(cfg.ScreenScraper))

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
