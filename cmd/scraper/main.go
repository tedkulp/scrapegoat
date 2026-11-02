package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gosimple/slug"
	"github.com/spf13/cobra"
	"github.com/tedkulp/scrapegoat/internal/artwork"
	_ "github.com/tedkulp/scrapegoat/internal/artwork/effects" // Register effects
	"github.com/tedkulp/scrapegoat/internal/downloader"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scraper"
	"github.com/tedkulp/scrapegoat/pkg/config"
)

var (
	version = "1.0.0"
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
	Short: "List all available platforms from ScreenScraper",
	Long: `Fetches and displays all available platforms/systems from ScreenScraper.fr.
Results are cached locally and refreshed every 12 hours.`,
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

var (
	// Scrape command flags
	platformName    string
	romDir          string
	gamelistDir     string
	mediaRootDir    string
	cacheDir        string
	workers         int
	dryRun          bool
	autoDelete      string // "true", "false", or "" (prompt user)
	artworkFlag     string // "true", "false", or "" (use config)
	cacheHours      int    // hours before cache expires (default: 12)
	forceRedownload bool
)

func init() {
	// Root persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml or ~/.config/scrapegoat/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Scrape command flags
	scrapeCmd.Flags().StringVarP(&platformName, "platform", "p", "", "platform/system name (required)")
	scrapeCmd.Flags().StringVarP(&romDir, "rom-dir", "r", "", "ROM directory path (required)")
	scrapeCmd.Flags().StringVarP(&gamelistDir, "gamelist-dir", "o", "", "directory for gamelist.xml output (default: same as rom-dir)")
	scrapeCmd.Flags().StringVarP(&mediaRootDir, "media-root-dir", "m", "", "root directory for media files (default: same as rom-dir)")
	scrapeCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "cache directory for downloaded media (default: ~/.scrapegoat)")
	scrapeCmd.Flags().IntVarP(&workers, "workers", "w", 4, "number of concurrent download workers")
	scrapeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "scan and query API but don't download or write files")
	scrapeCmd.Flags().StringVar(&autoDelete, "auto-delete", "", "automatically delete failed ROMs: true=delete, false=keep, unset=prompt (default: prompt)")
	scrapeCmd.Flags().StringVar(&artworkFlag, "artwork", "", "enable/disable artwork generation: true=enable, false=disable, unset=use config (default: use config)")
	scrapeCmd.Flags().IntVar(&cacheHours, "cache-hours", 12, "hours before cache expires for both platforms and ROM metadata (default: 12)")
	scrapeCmd.Flags().BoolVar(&forceRedownload, "force-redownload", false, "re-download media files even if they already exist in the destination directory")

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

	logInfo("Fetching platforms from ScreenScraper.fr...")

	// Create scraper client
	client := scraper.NewClient(cfg.ScreenScraper.Credentials())

	// Fetch systems list
	systems, err := client.GetSystemsList()
	if err != nil {
		log.Fatalf("Failed to fetch platforms: %v", err)
	}

	logVerbose("Raw systems count from API: %d", len(systems))
	if verbose && len(systems) > 0 {
		logVerbose("First system: ID=%d, Names=%+v", systems[0].ID, systems[0].Names)
	}

	logInfo("Found %d platforms:\n", len(systems))

	// Sort by ID for consistent output
	type platformInfo struct {
		id         int
		slug       string
		name       string
		extensions []string
	}

	platforms := make([]platformInfo, 0, len(systems))
	preferredRegions := []string{"us", "wor", "eu", "jp"}

	for _, system := range systems {
		fullName := system.GetPreferredName(preferredRegions)
		if fullName == "" {
			continue
		}

		// Take only the first name if multiple names are comma-separated
		displayName := fullName
		if idx := strings.Index(displayName, ","); idx > 0 {
			displayName = displayName[:idx]
		}

		// Create proper URL-safe slug (same logic as config package)
		// This handles special characters like ², é, etc.
		// Remove dashes to make slugs more compact (e.g., "n64dd" instead of "n64-dd")
		platformSlug := strings.ReplaceAll(slug.Make(fullName), "-", "")

		// If name has multiple comma-separated parts, find the shortest slug
		if strings.Contains(fullName, ",") {
			nameParts := strings.Split(fullName, ",")
			shortestSlug := strings.ReplaceAll(slug.Make(strings.TrimSpace(nameParts[0])), "-", "")

			for _, part := range nameParts[1:] {
				partSlug := strings.ReplaceAll(slug.Make(strings.TrimSpace(part)), "-", "")
				if len(partSlug) < len(shortestSlug) {
					shortestSlug = partSlug
				}
			}
			platformSlug = shortestSlug
		}

		// Use the preferred slug (alias) if one exists
		displaySlug := config.GetPreferredSlug(platformSlug)

		platforms = append(platforms, platformInfo{
			id:         system.ID,
			slug:       displaySlug,
			name:       displayName,
			extensions: system.Extensions,
		})
	}

	// Print platforms with slug column
	fmt.Printf("%-5s %-25s %-30s %s\n", "ID", "Slug", "Platform Name", "Extensions")
	fmt.Printf("%-5s %-25s %-30s %s\n", "-----", "-------------------------", "------------------------------", "----------")

	for _, p := range platforms {
		extList := strings.Join(p.extensions, ", ")
		fmt.Printf("%-5d %-25s %-30s %s\n", p.id, p.slug, p.name, extList)
	}

	logInfo("\nTo use a platform, specify the slug with the --platform flag.")
	logInfo("Example: scrapegoat scrape --platform nes --rom-dir /path/to/roms")
}

func runScraper(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set cache expiration time for both platforms and ROM metadata
	cfg.SetCacheExpiration(cacheHours)

	// Get platform configuration
	platform, err := cfg.GetPlatform(platformName)
	if err != nil {
		log.Fatalf("Failed to get platform configuration: %v", err)
	}

	// Expand template variables in paths
	romDir = expandPathTemplate(romDir, platformName)
	gamelistDir = expandPathTemplate(gamelistDir, platformName)
	mediaRootDir = expandPathTemplate(mediaRootDir, platformName)
	cacheDir = expandPathTemplate(cacheDir, platformName)

	// Set default directories to rom-dir if not specified
	if gamelistDir == "" {
		gamelistDir = romDir
	}

	if mediaRootDir == "" {
		mediaRootDir = romDir
	}

	// Use cache directory from config if not specified via flag
	if cacheDir == "" {
		cacheDir = cfg.Output.CacheDir
	}

	// Override artwork config with command-line flag if provided
	if artworkFlag != "" {
		artworkFlagLower := strings.ToLower(strings.TrimSpace(artworkFlag))
		if artworkFlagLower == "true" {
			cfg.Artwork.Enabled = true
		} else if artworkFlagLower == "false" {
			cfg.Artwork.Enabled = false
		}
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
	scraperClient := scraper.NewClient(cfg.ScreenScraper.Credentials(),
		scraper.WithCache(cacheDir, cfg.GetCacheExpiration()),
		scraper.WithDebug(verbose),
	)

	// Step 3: Initialize downloader
	var dl *downloader.Downloader
	if !dryRun {
		dl, err = downloader.NewDownloader(cacheDir, platformName, workers)
		if err != nil {
			log.Fatalf("Failed to create downloader: %v", err)
		}
		// Enable debug mode if verbose
		dl.SetDebug(verbose)
	}

	// Step 3.5: Initialize artwork compositor if enabled
	var compositor *artwork.Compositor
	if cfg.Artwork.Enabled && !dryRun {
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
	errorCount := 0
	var failedROMs []scanner.ROMFile // Track ROMs that failed to process

	for i, rom := range roms {
		logInfo("\n[%d/%d] Processing: %s", i+1, len(roms), rom.Filename)

		// Hash the ROM file
		logVerbose("  Computing hashes...")
		hashes, err := scraper.HashROMFile(rom.Path)
		if err != nil {
			logError("  Failed to hash ROM: %v", err)
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}
		logVerbose("  CRC32: %s, MD5: %s", hashes.CRC32, hashes.MD5)

		// Query ScreenScraper API (or get from cache)
		logVerbose("  Querying ScreenScraper API...")
		result, err := scraperClient.Lookup(platform.ID, rom.Filename, hashes)
		if err != nil {
			logError("  Failed to get game info: %v", err)
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}

		if result.Outcome == scraper.OutcomeNotFound {
			if result.Cached {
				logError("  Failed to get game info: game not found in ScreenScraper database (cached)")
			} else {
				logError("  Failed to get game info: game not found in ScreenScraper database")
			}
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}

		game := result.Game
		gameName := game.GetPreferredName([]string{"us", "wor", "eu", "jp"})
		if result.Cached {
			logInfo("  Found: %s (cached)", gameName)
		} else {
			logInfo("  Found: %s", gameName)
		}

		if result.Outcome == scraper.OutcomeNonGame {
			logInfo("  Skipping: Marked as non-game by ScreenScraper")
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}

		if dryRun {
			dryRunMediaFiles := buildMediaFileList(game, rom.Filename)
			_, dryRunToDownload := filterExistingMediaFiles(dryRunMediaFiles, mediaRootDir)
			logInfo("  [DRY RUN] Would download %d media files (%d already exist)", len(dryRunToDownload), len(dryRunMediaFiles)-len(dryRunToDownload))
			successCount++
			continue
		}

		// Get ROM name without extension for media filenames and artwork
		romName := strings.TrimSuffix(rom.Filename, filepath.Ext(rom.Filename))

		// Download media files
		allMediaFiles := buildMediaFileList(game, rom.Filename)

		downloadedFiles := make(map[string]string)
		mediaAPICallCount := 0
		existingCount := 0

		// Determine which files need downloading vs already present in final location
		mediaFiles := allMediaFiles
		if !forceRedownload {
			var alreadyExisting map[string]string
			alreadyExisting, mediaFiles = filterExistingMediaFiles(allMediaFiles, mediaRootDir)
			for mType, path := range alreadyExisting {
				downloadedFiles[mType] = path
				existingCount++
			}
		}

		logVerbose("  Downloading %d media files (%d already exist)...", len(mediaFiles), existingCount)

		if len(mediaFiles) > 0 {
			results := dl.Download(mediaFiles)
			cachedCount := 0
			downloadedCount := 0

			for _, result := range results {
				if result.Error != nil {
					logVerbose("    Failed to download %s: %v", result.MediaFile.Type, result.Error)
					continue
				}

				if result.Cached {
					cachedCount++
					logVerbose("    Cached: %s", result.MediaFile.Type)
				} else {
					downloadedCount++
					mediaAPICallCount++
					logVerbose("    Downloaded: %s", result.MediaFile.Type)
				}

				typeDir := filepath.Join(mediaRootDir, result.MediaFile.Type)
				finalPath := filepath.Join(typeDir, result.MediaFile.Filename)
				if err := dl.CopyToFinal(result.LocalPath, finalPath); err != nil {
					logVerbose("    Failed to copy %s to final location: %v", result.MediaFile.Type, err)
					continue
				}

				downloadedFiles[result.MediaFile.Type] = finalPath
			}

			// Build media summary showing only non-zero counts
			var parts []string
			if existingCount > 0 {
				parts = append(parts, fmt.Sprintf("%d existing", existingCount))
			}
			if downloadedCount > 0 {
				parts = append(parts, fmt.Sprintf("%d downloaded", downloadedCount))
			}
			if cachedCount > 0 {
				parts = append(parts, fmt.Sprintf("%d from cache", cachedCount))
			}
			if len(parts) > 0 {
				logInfo("  Media: %s", strings.Join(parts, ", "))
			}
		} else if existingCount > 0 {
			logInfo("  Media: %d existing", existingCount)
		}

		// Generate artwork if enabled
		if compositor != nil && len(downloadedFiles) > 0 {
			logVerbose("  Generating artwork...")
			artworkDir := filepath.Join(mediaRootDir, cfg.Artwork.OutputDir)

			// Convert downloadedFiles to artwork.MediaFiles format (just type name without directory)
			artworkMediaFiles := make(artwork.MediaFiles)
			for mediaType, fullPath := range downloadedFiles {
				artworkMediaFiles[mediaType] = fullPath
			}

			artworkFiles, err := compositor.GenerateArtwork(romName, artworkMediaFiles, artworkDir)
			if err != nil {
				logVerbose("  Warning: Failed to generate artwork: %v", err)
			} else {
				logVerbose("  Generated %d artwork files", len(artworkFiles))
				// Add artwork files to downloadedFiles so they're included in gamelist.xml
				for artType, artPath := range artworkFiles {
					downloadedFiles[artType] = artPath
				}
			}
		}

		// Add game to metadata
		if err := gen.AddGameFromScraper(rom.Path, game, downloadedFiles); err != nil {
			logError("  Failed to add game metadata: %v", err)
			errorCount++
			failedROMs = append(failedROMs, rom)
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

	// Handle failed ROMs - offer to delete them
	if len(failedROMs) > 0 && !dryRun {
		logInfo("\n=== Failed ROMs ===")
		logInfo("The following ROMs failed to process:")
		for i, rom := range failedROMs {
			logInfo("  %d. %s", i+1, rom.Filename)
		}

		// Determine whether to delete based on auto-delete flag
		var shouldDelete bool
		autoDeleteLower := strings.ToLower(strings.TrimSpace(autoDelete))

		if autoDeleteLower == "true" {
			// Auto-delete enabled
			shouldDelete = true
			logInfo("\nAuto-deleting failed ROM files (--auto-delete=true)...")
		} else if autoDeleteLower == "false" {
			// Auto-delete explicitly disabled
			shouldDelete = false
			logInfo("\nKeeping failed ROM files (--auto-delete=false)")
		} else {
			// Prompt user (default behavior)
			logInfo("\nWould you like to delete these ROM files? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			shouldDelete = (response == "y" || response == "yes")
		}

		if shouldDelete {
			if autoDeleteLower != "true" {
				logInfo("\nDeleting failed ROM files...")
			}
			deletedCount := 0
			deleteErrors := 0

			for _, rom := range failedROMs {
				if err := os.Remove(rom.Path); err != nil {
					logError("  Failed to delete %s: %v", rom.Filename, err)
					deleteErrors++
				} else {
					logInfo("  Deleted: %s", rom.Filename)
					deletedCount++
				}
			}

			logInfo("\nDeletion complete: %d deleted, %d errors", deletedCount, deleteErrors)
		} else {
			if autoDeleteLower != "false" {
				logInfo("\nSkipping deletion. Failed ROM files were kept.")
			}
		}
	}

	// Summary
	logInfo("\n=== Summary ===")
	logInfo("Total ROMs processed: %d", len(roms))
	logInfo("Successful: %d", successCount)
	logInfo("Failed: %d", errorCount)

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
		logInfo("%s", cacheDurationMessage(cfg.GetCacheExpiration()))
	}
}

func cacheDurationMessage(hours int) string {
	unit := "hours"
	if hours == 1 {
		unit = "hour"
	}
	return fmt.Sprintf("Game data is cached for %d %s", hours, unit)
}

func filterExistingMediaFiles(mediaFiles []downloader.MediaFile, mediaRootDir string) (existing map[string]string, toDownload []downloader.MediaFile) {
	existing = make(map[string]string)
	for _, mf := range mediaFiles {
		finalPath := filepath.Join(mediaRootDir, mf.Type, mf.Filename)
		if _, err := os.Stat(finalPath); err == nil {
			existing[mf.Type] = finalPath
		} else {
			toDownload = append(toDownload, mf)
		}
	}
	return
}

func buildMediaFileList(game *scraper.Game, romFilename string) []downloader.MediaFile {
	var mediaFiles []downloader.MediaFile
	preferredRegions := []string{"us", "wor", "eu", "jp"}

	// Get ROM name without extension for media filenames
	romName := strings.TrimSuffix(romFilename, filepath.Ext(romFilename))

	// Map ScreenScraper media types to EmulationStation directory names
	// This mapping is required by EmulationStation's media organization
	mediaTypeMapping := map[string]string{
		"box-3D":      "3dboxes",
		"box-2D":      "box2dfront",
		"box-2D-back": "backcovers",
		"manuel":      "manuals",
		"ss":          "screenshots",
		"sstitle":     "titlescreens",
		"wheel":       "wheel",
		"video":       "videos",
		"support-2D":  "physicalmedia",
	}

	for apiType, esDirectory := range mediaTypeMapping {
		media := game.GetPreferredMedia(apiType, preferredRegions)
		if media != nil && media.URL != "" {
			// Determine file extension from format or URL
			ext := media.Format
			if ext == "" {
				ext = filepath.Ext(media.URL)
			}
			if ext != "" && !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}

			// Files are organized by EmulationStation media type in subdirectories
			mediaFiles = append(mediaFiles, downloader.MediaFile{
				URL:      media.URL,
				Filename: fmt.Sprintf("%s%s", romName, ext),
				Type:     esDirectory, // Use EmulationStation directory name
			})
		}
	}

	return mediaFiles
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

// expandPathTemplate replaces template variables in a path string
// Currently supports: {platform}
func expandPathTemplate(path string, platformName string) string {
	// Replace {platform} with the actual platform name
	path = strings.ReplaceAll(path, "{platform}", platformName)
	return path
}

// quotaLines formats the quota and failed-request quota lines shown by user-info.
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
	client := scraper.NewClient(cfg.ScreenScraper.Credentials())

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
