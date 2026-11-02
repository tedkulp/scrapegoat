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
3. **Lookup** - `scraper.Client.Lookup()` asks ScreenScraper (or the local cache) which game a ROM is, by hashes and system ID. It returns a `LookupResult` whose `Outcome` is `OutcomeMatch`, `OutcomeNotFound` or `OutcomeNonGame`; an `error` means no answer could be obtained. Retry, backoff, throttling and quota tracking live inside the client.
4. **Download Media** - `downloader.Downloader` downloads media files (box art, screenshots, wheel, video) concurrently using worker pool
5. **Generate Metadata** - `metadata.Generator` builds gamelist.xml with relative paths compatible with EmulationStation
6. **Write Output** - Writes gamelist.xml and copies media files to final location

### Key Design Patterns

**ZIP Handling** (`internal/scraper/hasher.go:72-144`): When hashing ZIP files, the hasher extracts the ROM file inside and hashes that content, not the ZIP archive itself. This ensures accurate matching against ScreenScraper's database which uses ROM hashes, not archive hashes.

**Region/Language Preferences** (`internal/scraper/types.go:103-190`): The `Game` type includes methods like `GetPreferredName()`, `GetPreferredMedia()`, and `GetPreferredSynopsis()` that accept preferred region/language lists and fall back through the list until a match is found.

**Concurrent Downloads** (`internal/downloader/downloader.go:50-84`): Uses a worker pool pattern with channels. Jobs are sent to workers via a channel, results collected via another channel. The number of workers is configurable via `--workers` flag.

**API Retry Logic** (`internal/scraper/client.go`, `getJSONWithRetry`, used by `Lookup`, `GetSystemsList` and `GetUserInfo`): The ScreenScraper client makes up to 3 attempts per request, with exponential backoff between attempts (2s, then 4s). A rate limit response (429) also triggers a 10-second delay, skipped on the final attempt. Only failures that can clear up on their own are retried: transport errors, body-read errors, JSON decode errors (an overloaded server often returns HTML), 401 (API closed to non-members because the server is overloaded), 429 (thread limit) and any 5xx. Every other status is permanent and returns a `*StatusError` after the first request, naming the cause: 400 (malformed request), 403 (bad developer credentials), 404, 423 (API fully closed), 426 (software name blacklisted), 430 (daily quota exceeded), 431 (failed-request quota exceeded). A 404 from `jeuInfos.php` means no match and becomes the `OutcomeNotFound` outcome (cached, no error), as does a decoded empty "game not found" response. Status meanings are from ScreenScraper's API v2 docs, https://www.screenscraper.fr/webapi2.php.

**Lookup Client Construction** (`internal/scraper/client.go`): `NewClient(creds Credentials, opts ...Option)`. Options: `WithHTTPClient`, `WithClock` (one injectable clock for backoff, throttle and cache expiry), `WithCache`, `WithThrottle`, `WithDebug`, `WithDebugOutput`. `Client.Quota()` returns the numeric quota from the latest `ssuser` response; `Client.RequestCount()` counts real HTTP requests, retries included. Debug logging prints each request URL with `devpassword` and `sspassword` replaced by `***`.

**Relative Path Generation** (`internal/metadata/gamelist.go:45-109`): Media file paths in gamelist.xml are stored relative to the ROM directory with "./" prefix, as required by EmulationStation.

**Platform Caching** (`pkg/config/config.go:137-245`): Platform definitions are fetched from the ScreenScraper API via `systemesListe.php` and cached locally. The cache expires after 12 hours. When a platform is requested, the system first checks `config.yaml`, then the cache, and finally fetches from the API if needed. This allows the application to work with any platform supported by ScreenScraper without manual configuration.

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- Standard library for HTTP, XML parsing, hashing, and ZIP handling

## API Considerations

**ScreenScraper Rate Limits**: The API enforces rate limits. The scraper includes:
- A 1 second throttle between consecutive HTTP requests, enforced inside the client (`WithThrottle`); cache hits make no request and do not wait
- Retry logic with exponential backoff
- 10-second delay on rate limit responses

**Authentication**: All API requests require both developer credentials (dev_id, dev_password) and user credentials (user_id, user_password).

**Hash-Based Matching**: The API primarily matches ROMs by their CRC32, MD5, and SHA1 hashes. Filename (`romnom`) is sent as fallback but hash matching is most reliable.

## Output Format

Generates EmulationStation-compatible output:
- `gamelist.xml` - XML file with game metadata (names, descriptions, ratings, media paths)
- `media/` directory - Downloaded images and videos with naming pattern `{rom-name}-{type}.{ext}`

Media types downloaded: box-2D (box art), screenshot-title (screenshot), wheel (logo), video.

## Agent skills

### Issue tracker

Issues live in GitHub Issues on tedkulp/scrapegoat (via `gh`). See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-role vocabulary (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
