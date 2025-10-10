package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tedkulp/scrapegoat/internal/downloader"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scraper"
	"github.com/tedkulp/scrapegoat/pkg/config"
)

var (
	cfgFile      string
	platformName string
	romDir       string
	gamelistDir  string
	mediaRootDir string
	cacheDir     string
	workers      int
	dryRun       bool
	verbose      bool
)

var rootCmd = &cobra.Command{
	Use:   "scrapegoat",
	Short: "ROM scraper for EmulationStation using ScreenScraper.fr API",
	Long: `Scrapegoat is a CLI tool that scrapes ROM files using the ScreenScraper.fr API.
It downloads game metadata and media files, then generates a gamelist.xml file
compatible with EmulationStation.`,
	Run: runScraper,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml or ~/.config/scrapegoat/config.yaml)")
	rootCmd.Flags().StringVarP(&platformName, "platform", "p", "", "platform/system name (required)")
	rootCmd.Flags().StringVarP(&romDir, "rom-dir", "r", "", "ROM directory path (required)")
	rootCmd.Flags().StringVarP(&gamelistDir, "gamelist-dir", "o", "", "directory for gamelist.xml output (default: same as rom-dir)")
	rootCmd.Flags().StringVarP(&mediaRootDir, "media-root-dir", "m", "", "root directory for media files (default: same as rom-dir)")
	rootCmd.Flags().StringVar(&cacheDir, "cache-dir", "", "cache directory for downloaded media (default: ~/.scrapegoat)")
	rootCmd.Flags().IntVarP(&workers, "workers", "w", 4, "number of concurrent download workers")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "scan and query API but don't download or write files")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.MarkFlagRequired("platform")
	rootCmd.MarkFlagRequired("rom-dir")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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

	// Step 3: Initialize downloader
	var dl *downloader.Downloader
	if !dryRun {
		dl, err = downloader.NewDownloader(cacheDir, platformName, workers)
		if err != nil {
			log.Fatalf("Failed to create downloader: %v", err)
		}
	}

	// Step 4: Initialize metadata generator
	// Note: mediaRootDir is used directly, no longer needs cfg.Output.MediaDir subdirectory
	gen := metadata.NewGenerator(gamelistDir, mediaRootDir)

	// Step 5: Process each ROM
	logInfo("\nProcessing ROM files...")
	successCount := 0
	errorCount := 0

	for i, rom := range roms {
		logInfo("\n[%d/%d] Processing: %s", i+1, len(roms), rom.Filename)

		// Hash the ROM file
		logVerbose("  Computing hashes...")
		hashes, err := scraper.HashROMFile(rom.Path)
		if err != nil {
			logError("  Failed to hash ROM: %v", err)
			errorCount++
			continue
		}
		logVerbose("  CRC32: %s, MD5: %s", hashes.CRC32, hashes.MD5)

		// Query ScreenScraper API
		logVerbose("  Querying ScreenScraper API...")
		if verbose {
			debugURL := scraperClient.GetDebugURL(platform.ID, rom.Filename, hashes)
			logVerbose("  API URL: %s", debugURL)
		}
		game, err := scraperClient.GetGameInfo(platform.ID, rom.Filename, hashes)
		if err != nil {
			logError("  Failed to get game info: %v", err)
			errorCount++
			continue
		}

		gameName := game.GetPreferredName([]string{"us", "wor", "eu", "jp"})
		logInfo("  Found: %s", gameName)

		if dryRun {
			logInfo("  [DRY RUN] Would download %d media files", len(game.Medias))
			successCount++
			continue
		}

		// Download media files
		mediaFiles := buildMediaFileList(game, rom.Filename)
		logVerbose("  Downloading %d media files...", len(mediaFiles))

		downloadedFiles := make(map[string]string)
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
			continue
		}

		successCount++

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

	// Summary
	logInfo("\n=== Summary ===")
	logInfo("Total ROMs processed: %d", len(roms))
	logInfo("Successful: %d", successCount)
	logInfo("Failed: %d", errorCount)

	if dryRun {
		logInfo("\nDRY RUN completed - no files were written")
	} else {
		logInfo("\nScraping complete!")
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
