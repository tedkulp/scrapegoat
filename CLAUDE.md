# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scrapegoat is a ROM scraper for EmulationStation that uses the ScreenScraper.fr API to gather game metadata and media files. It identifies ROMs using CRC32, MD5, and SHA1 hashes for accurate matching, handles ZIP archives transparently, and downloads media files concurrently.

## Build and Run

```bash
# Build the binary
go build -o scrapegoat cmd/scraper/main.go

# List available platforms
./scrapegoat list-platforms

# Scrape ROMs with basic options
./scrapegoat scrape --platform nes --rom-dir /path/to/roms

# Dry run (test without downloading)
./scrapegoat scrape --platform nes --rom-dir /path/to/roms --dry-run

# With verbose output
./scrapegoat scrape --platform nes --rom-dir /path/to/roms --verbose
```

## Configuration

The application uses `config.yaml` for configuration. Copy `config.yaml.example` to `config.yaml` and fill in ScreenScraper.fr API credentials:
- `screenscraper.dev_id` and `screenscraper.dev_password` - Developer credentials
- `screenscraper.user_id` and `screenscraper.user_password` - User credentials

**Platform Auto-Discovery**: Platforms are automatically fetched from the ScreenScraper.fr API and cached locally in `~/.scrapegoat/platforms.json`. The cache is refreshed every 12 hours. You can optionally define platforms manually in `config.yaml` to override API data or work offline.

## Architecture

### Package Structure

- `cmd/scraper/main.go` - CLI entry point using Cobra. Orchestrates the scraping workflow.
- `pkg/config/` - Configuration loading via Viper. Handles YAML config parsing and validation.
- `internal/scanner/` - ROM file discovery. Walks directories to find files with configured extensions.
- `internal/scraper/` - ScreenScraper API client and ROM hashing.
- `internal/downloader/` - Concurrent media file downloads with worker pool.
- `internal/metadata/` - gamelist.xml generation for EmulationStation.

### Workflow (cmd/scraper/main.go:60-225)

The main scraping workflow follows these steps:

1. **Scan** - `scanner.Scanner` walks the ROM directory to find files matching platform extensions
2. **Hash** - `scraper.HashROMFile()` computes CRC32, MD5, SHA1 for each ROM. For ZIP files, extracts and hashes the ROM inside (not the ZIP container).
3. **Query API** - `scraper.Client.GetGameInfo()` queries ScreenScraper with hashes and system ID. Includes retry logic with exponential backoff for rate limiting.
4. **Download Media** - `downloader.Downloader` downloads media files (box art, screenshots, wheel, video) concurrently using worker pool
5. **Generate Metadata** - `metadata.Generator` builds gamelist.xml with relative paths compatible with EmulationStation
6. **Write Output** - Writes gamelist.xml and copies media files to final location

### Key Design Patterns

**ZIP Handling** (`internal/scraper/hasher.go:72-144`): When hashing ZIP files, the hasher extracts the ROM file inside and hashes that content, not the ZIP archive itself. This ensures accurate matching against ScreenScraper's database which uses ROM hashes, not archive hashes.

**Region/Language Preferences** (`internal/scraper/types.go:103-190`): The `Game` type includes methods like `GetPreferredName()`, `GetPreferredMedia()`, and `GetPreferredSynopsis()` that accept preferred region/language lists and fall back through the list until a match is found.

**Concurrent Downloads** (`internal/downloader/downloader.go:50-84`): Uses a worker pool pattern with channels. Jobs are sent to workers via a channel, results collected via another channel. The number of workers is configurable via `--workers` flag.

**API Retry Logic** (`internal/scraper/client.go:67-98`): The ScreenScraper client retries failed requests up to 3 times with exponential backoff (2s, 4s, 8s). Rate limit responses (429) trigger a 10-second delay before retry.

**Relative Path Generation** (`internal/metadata/gamelist.go:45-109`): Media file paths in gamelist.xml are stored relative to the ROM directory with "./" prefix, as required by EmulationStation.

**Platform Caching** (`pkg/config/config.go:137-245`): Platform definitions are fetched from the ScreenScraper API via `systemesListe.php` and cached locally. The cache expires after 12 hours. When a platform is requested, the system first checks `config.yaml`, then the cache, and finally fetches from the API if needed. This allows the application to work with any platform supported by ScreenScraper without manual configuration.

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- Standard library for HTTP, XML parsing, hashing, and ZIP handling

## API Considerations

**ScreenScraper Rate Limits**: The API enforces rate limits. The scraper includes:
- 1 second delay between ROM queries (cmd/scraper/main.go:200)
- Retry logic with exponential backoff
- 10-second delay on rate limit responses

**Authentication**: All API requests require both developer credentials (dev_id, dev_password) and user credentials (user_id, user_password).

**Hash-Based Matching**: The API primarily matches ROMs by their CRC32, MD5, and SHA1 hashes. Filename (`romnom`) is sent as fallback but hash matching is most reliable.

## Output Format

Generates EmulationStation-compatible output:
- `gamelist.xml` - XML file with game metadata (names, descriptions, ratings, media paths)
- `media/` directory - Downloaded images and videos with naming pattern `{rom-name}-{type}.{ext}`

Media types downloaded: box-2D (box art), screenshot-title (screenshot), wheel (logo), video.
