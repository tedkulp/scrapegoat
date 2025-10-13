package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/spf13/cobra"
	"github.com/tedkulp/scrapegoat/internal/downloader"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scraper"
	"github.com/tedkulp/scrapegoat/pkg/config"
)

var (
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

var (
	// Scrape command flags
	platformName string
	romDir       string
	gamelistDir  string
	mediaRootDir string
	cacheDir     string
	workers      int
	dryRun       bool
	autoDelete   string // "true", "false", or "" (prompt user)
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

	scrapeCmd.MarkFlagRequired("platform")
	scrapeCmd.MarkFlagRequired("rom-dir")

	// Add subcommands
	rootCmd.AddCommand(scrapeCmd)
	rootCmd.AddCommand(listPlatformsCmd)
	rootCmd.AddCommand(userInfoCmd)
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
	client := scraper.NewClient(
		cfg.ScreenScraper.DevID,
		cfg.ScreenScraper.DevPassword,
		cfg.ScreenScraper.UserID,
		cfg.ScreenScraper.UserPassword,
		cfg.ScreenScraper.SoftwareName,
	)

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
	scraperClient := scraper.NewClient(
		cfg.ScreenScraper.DevID,
		cfg.ScreenScraper.DevPassword,
		cfg.ScreenScraper.UserID,
		cfg.ScreenScraper.UserPassword,
		cfg.ScreenScraper.SoftwareName,
	)

	// Enable caching
	scraperClient.SetCache(cacheDir)

	// Enable debug mode if verbose
	scraperClient.SetDebug(verbose)

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

	// Step 4: Initialize metadata generator
	// Note: mediaRootDir is used directly, no longer needs cfg.Output.MediaDir subdirectory
	gen := metadata.NewGenerator(gamelistDir, mediaRootDir)

	// Step 5: Process each ROM
	logInfo("\nProcessing ROM files...")
	successCount := 0
	errorCount := 0
	var userInfo *scraper.UserInfo
	var initialRequests int
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
		if verbose {
			debugURL := scraperClient.GetDebugURL(platform.ID, rom.Filename, hashes)
			logVerbose("  API URL: %s", debugURL)
		}
		game, freshUserInfo, cached, err := scraperClient.GetGameInfo(platform.ID, rom.Filename, hashes)
		if err != nil {
			logError("  Failed to get game info: %v", err)
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}

		// Update userInfo if we got fresh data from API (not cached)
		if !cached && freshUserInfo != nil {
			if userInfo == nil {
				// First ROM - record initial request count
				fmt.Sscanf(freshUserInfo.RequestsToday, "%d", &initialRequests)
			}
			userInfo = freshUserInfo
		}

		gameName := game.GetPreferredName([]string{"us", "wor", "eu", "jp"})
		if cached {
			logInfo("  Found: %s (cached)", gameName)
		} else {
			logInfo("  Found: %s", gameName)
		}

		// Check if this is marked as a non-game by ScreenScraper
		if game.IsNonGame() {
			logInfo("  Skipping: Marked as non-game by ScreenScraper")
			errorCount++
			failedROMs = append(failedROMs, rom)
			continue
		}

		if dryRun {
			logInfo("  [DRY RUN] Would download %d media files", len(game.Medias))
			successCount++
			continue
		}

		// Download media files
		mediaFiles := buildMediaFileList(game, rom.Filename)
		logVerbose("  Downloading %d media files...", len(mediaFiles))

		downloadedFiles := make(map[string]string)
		mediaAPICallCount := 0
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
					mediaAPICallCount++ // Each non-cached media download counts as an API call
					logVerbose("    Downloaded: %s", result.MediaFile.Type)
				}

				// Copy to final media directory, organized by type
				// mediaRootDir/{type}/{filename} e.g., /media/wheel/Game.png
				typeDir := filepath.Join(mediaRootDir, result.MediaFile.Type)
				finalPath := filepath.Join(typeDir, result.MediaFile.Filename)
				if err := dl.CopyToFinal(result.LocalPath, finalPath); err != nil {
					logVerbose("    Failed to copy %s to final location: %v", result.MediaFile.Type, err)
					continue
				}

				downloadedFiles[result.MediaFile.Type] = finalPath
			}

			// Summary of media operations
			if cachedCount > 0 && downloadedCount > 0 {
				logInfo("  Media: %d downloaded, %d from cache", downloadedCount, cachedCount)
			} else if cachedCount > 0 {
				logInfo("  Media: %d from cache", cachedCount)
			} else if downloadedCount > 0 {
				logInfo("  Media: %d downloaded", downloadedCount)
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

		// Display real-time API quota if available
		if userInfo != nil {
			var requestsToday, maxRequests int
			fmt.Sscanf(userInfo.RequestsToday, "%d", &requestsToday)
			fmt.Sscanf(userInfo.MaxRequestsPerDay, "%d", &maxRequests)
			remaining := maxRequests - requestsToday
			logInfo("  API Quota: %d requests remaining today", remaining)
		}

		// Be nice to the API - add a small delay between requests
		time.Sleep(1 * time.Second)
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
	if userInfo != nil && !dryRun {
		var finalRequests int
		fmt.Sscanf(userInfo.RequestsToday, "%d", &finalRequests)
		totalAPICalls := finalRequests - initialRequests

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
		logInfo("Game data is cached for 12 hours")
	}
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
		"ss-title":    "titlescreens",
		"wheel":       "wheel",
		"video":       "videos",
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

func runUserInfo(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logInfo("Fetching user information from ScreenScraper.fr...")

	// Create scraper client
	client := scraper.NewClient(
		cfg.ScreenScraper.DevID,
		cfg.ScreenScraper.DevPassword,
		cfg.ScreenScraper.UserID,
		cfg.ScreenScraper.UserPassword,
		cfg.ScreenScraper.SoftwareName,
	)

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
	logInfo("Requests Today: %s", userInfo.RequestsToday)
	logInfo("Max Requests Per Day: %s", userInfo.MaxRequestsPerDay)
	logInfo("Max Requests Per Hour: %s", userInfo.MaxRequestsPerHour)
	logInfo("Max Requests Per Minute: %s", userInfo.MaxRequestsPerMinute)
	logInfo("Max Requests Per Second: %s", userInfo.MaxRequestsPerSecond)
	logInfo("Max Threads: %s", userInfo.MaxThreads)

	// Calculate remaining requests
	if userInfo.RequestsToday != "" && userInfo.MaxRequestsPerDay != "" {
		var requestsToday, maxRequests int
		fmt.Sscanf(userInfo.RequestsToday, "%d", &requestsToday)
		fmt.Sscanf(userInfo.MaxRequestsPerDay, "%d", &maxRequests)
		remaining := maxRequests - requestsToday
		logInfo("\nRemaining Requests Today: %d", remaining)
	}
}
