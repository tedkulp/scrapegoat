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

# Dry run (queries the API, downloads and writes nothing)
./scrapegoat scrape --platform nes --rom-dir /path/to/roms --dry-run

# Real scrape, keeping unmatched ROMs without a prompt
./scrapegoat scrape --platform nes --rom-dir /path/to/roms --auto-delete=false --verbose
```

Pass `--auto-delete=false` whenever you run `scrape`: by default the run ends by prompting to delete every ROM that was not found, was marked non-game, or failed, and `--auto-delete=true` deletes them without asking. `./scrapegoat scrape --help` lists every flag.

`go install ./cmd/scraper` (and `make install`) names the binary `scraper`, after the package directory; `go build -o scrapegoat` and the release builds (`.goreleaser.yml`) name it `scrapegoat`.

## User-facing docs

`README.md` (full reference: flags, output layout, caching, troubleshooting), `QUICKSTART.md` (first run) and `ARTWORK.md` (artwork generation) describe CLI behavior to users. When a change touches a command, flag, default, output path or cache location, update those docs in the same change.

## Configuration

The application uses `config.yaml` for configuration, looked up as `./config.yaml`, then `~/.config/scrapegoat/config.yaml`, or the path given by `--config`. Copy `config.yaml.example` to `config.yaml` and fill in ScreenScraper.fr API credentials:
- `screenscraper.dev_id` and `screenscraper.dev_password` - Developer credentials
- `screenscraper.user_id` and `screenscraper.user_password` - User credentials

**Platform Auto-Discovery**: Platforms are automatically fetched from the ScreenScraper.fr API and cached locally in `<cache-dir>/platforms.json` (default `~/.scrapegoat`). The cache expires after `--cache-hours` (default 12). You can optionally define platforms manually in `config.yaml` to override API data or work offline.

## Architecture

### Package Structure

- `cmd/scraper/main.go` - CLI entry point using Cobra. Orchestrates the scraping workflow.
- `pkg/config/` - Configuration loading via Viper. Handles YAML config parsing and validation.
- `internal/scanner/` - ROM file discovery. Walks directories to find files with configured extensions.
- `internal/scraper/` - ScreenScraper API client, game cache and ROM hashing.
- `internal/media/` - Media placement: picks one file per media type, plans where each comes from (media root, media cache or ScreenScraper), and fetches and writes them with a worker pool.
- `internal/metadata/` - gamelist.xml generation for EmulationStation.
- `internal/artwork/` - Optional composed artwork (miximages) built from downloaded media, enabled by `artwork.enabled` or `--artwork`.

### Workflow (`runScraper` in cmd/scraper/main.go)

The main scraping workflow follows these steps:

1. **Scan** - `scanner.Scanner` walks the ROM directory to find files matching platform extensions
2. **Hash** - `scraper.HashROMFile()` computes CRC32, MD5, SHA1 for each ROM. For ZIP files, extracts and hashes the ROM inside (not the ZIP container).
3. **Lookup** - `scraper.Client.Lookup()` asks ScreenScraper (or the local cache) which game a ROM is, by hashes and system ID. It returns a `LookupResult` whose `Outcome` is `OutcomeMatch`, `OutcomeNotFound` or `OutcomeNonGame`; an `error` means no answer could be obtained. Retry, backoff, throttling and quota tracking live inside the client.
4. **Download Media** - `media.Placer.Plan` builds the ROM's media plan: one file per media type, each marked existing (already in `<media-root-dir>/<type>/`), from the media cache, or to download. `--force-redownload` marks every file to download. Dry-run reports the plan; otherwise `media.Placer.Execute` carries it out and returns a `media.Result` (placed paths plus existing / from-cache / downloaded / failed counts). A failed media file never fails the ROM.
5. **Artwork** - when enabled, `artwork.Compositor` composes images from the downloaded media
6. **Generate Metadata** - `metadata.Generator` builds gamelist.xml and writes it to `--gamelist-dir`
7. **Unmatched ROMs** - ROMs that were not found, non-game, or failed are listed, and `--auto-delete` decides whether they are deleted (unset means prompt)

### Key Design Patterns

**ZIP Handling** (`hashZippedROM` in `internal/scraper/hasher.go`): When hashing ZIP files, the hasher extracts the ROM file inside and hashes that content, not the ZIP archive itself. This ensures accurate matching against ScreenScraper's database which uses ROM hashes, not archive hashes.

**Region/Language Preferences** (`internal/scraper/types.go`): The `Game` type includes methods like `GetPreferredName()`, `GetPreferredMedia()`, and `GetPreferredSynopsis()` that accept preferred region/language lists and fall back through the list until a match is found. `scraper.DefaultRegions()` is the one definition of the default region order (us, wor, eu, jp); callers use it rather than writing the list out.

**Media Placement** (`internal/media/media.go`): `media.New(cacheDir, platform, mediaRoot, workers, opts...)` with options `WithHTTPClient` and `WithDebugOutput`; the package prints nothing else, and the CLI does the logging. `Plan` makes no requests and no writes. `Execute` fetches one ROM's files concurrently with a worker pool (`--workers`); ROMs stay sequential. Every write to the media cache and media root goes to a temp file in the same directory and is renamed into place. Each cached file has a media identity sidecar `<file>.json` holding the `systemeid`, `jeuid` and `media` query parameters of its ScreenScraper URL (never credentials or the host): a matching sidecar means the cached file is used, a different one means it is downloaded again, a missing one means the file is trusted and a sidecar written (adopted), and a URL without `jeuid` or `media` means the cached file is trusted with no sidecar. A zero-byte cached file counts as missing. The media root has an existence-only check and no sidecars. See `docs/adr/0001-media-cache-identity-sidecar.md`. `media.Types()` is the one list of media type folders; artwork layer validation (`artwork.MediaTypes`) uses it.

**API Retry Logic** (`internal/scraper/client.go`, `getJSONWithRetry`, used by `Lookup`, `GetSystemsList` and `GetUserInfo`): The ScreenScraper client makes up to 3 attempts per request, with exponential backoff between attempts (2s, then 4s). A rate limit response (429) also triggers a 10-second delay, skipped on the final attempt. Only failures that can clear up on their own are retried: transport errors, body-read errors, JSON decode errors (an overloaded server often returns HTML), 401 (API closed to non-members because the server is overloaded), 408 (request timeout), 429 (thread limit) and any 5xx. Every other status is permanent and returns a `*StatusError` after the first request, naming the cause: 400 (malformed request), 403 (bad developer credentials), 404, 423 (API fully closed), 426 (software name blacklisted), 430 (daily quota exceeded), 431 (failed-request quota exceeded). A 404 from `jeuInfos.php` means ScreenScraper has no entry for the ROM and becomes the `OutcomeNotFound` outcome (cached, no error), as does a decoded empty "game not found" response. Status meanings are from ScreenScraper's API v2 docs, https://www.screenscraper.fr/webapi2.php.

**Lookup Client Construction** (`internal/scraper/client.go`): `NewClient(creds Credentials, opts ...Option)`. Options: `WithHTTPClient`, `WithClock` (one injectable clock for backoff, throttle and cache expiry), `WithCache`, `WithThrottle`, `WithDebug`, `WithDebugOutput`. `Client.Quota()` returns the numeric quota from the latest `ssuser` response; `Client.RequestCount()` counts real HTTP requests, retries included. Debug logging prints each request URL with `devpassword` and `sspassword` replaced by `***`.

**gamelist.xml Paths** (`Generator.AddGameFromScraper` in `internal/metadata/gamelist.go`): Each entry's `<path>` is the ROM filename with a "./" prefix, as EmulationStation expects. Media tags are omitted: EmulationStation finds media by directory and ROM filename.

**Platform Caching** (`Config.GetPlatform` in `pkg/config/config.go`): Platform definitions are fetched from the ScreenScraper API via `systemesListe.php` and cached locally. The cache expires after `--cache-hours` (default 12). When a platform is requested, the system first checks `config.yaml`, then the cache, and finally fetches from the API if needed. Cache lookups by slug are case-insensitive. This allows the application to work with any platform supported by ScreenScraper without manual configuration.

**Caches** (all under `--cache-dir`, default `~/.scrapegoat`): `platforms.json` and `games/` (game lookups, including not-found results) expire after `--cache-hours`; media in `cache/<platform>/<media-type>/` (with a `.json` media identity sidecar per file) never expires.

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/disintegration/imaging` - Image processing for artwork generation
- Standard library for HTTP, XML parsing, hashing, and ZIP handling

## API Considerations

**ScreenScraper Rate Limits**: The API enforces rate limits. The scraper includes:
- A 1 second throttle between consecutive HTTP requests, enforced inside the client (`WithThrottle`); cache hits make no request and do not wait
- Retry logic with exponential backoff
- 10-second delay on rate limit responses

Media downloads also go through ScreenScraper and count against the daily quota.

**Authentication**: All API requests require both developer credentials (dev_id, dev_password) and user credentials (user_id, user_password).

**Hash-Based Matching**: The API primarily matches ROMs by their CRC32, MD5, and SHA1 hashes. Filename (`romnom`) is sent as fallback but hash matching is most reliable.

## Output Format

Generates EmulationStation-compatible output:
- `gamelist.xml` in `--gamelist-dir` (default: rom-dir) - names, descriptions, ratings, release dates, developers, publishers, genres and player counts
- Media in `<media-root-dir>/<type>/<rom-name-without-extension>.<ext>` (default media root: rom-dir)

Media type directories, mapped from ScreenScraper types in `internal/media` (`typeMapping`): `3dboxes` (box-3D), `box2dfront` (box-2D), `backcovers` (box-2D-back), `manuals` (manuel), `screenshots` (ss), `titlescreens` (sstitle), `wheel` (wheel), `videos` (video), `physicalmedia` (support-2D). Media regions are preferred in the order us, wor, eu, jp.

## Agent skills

### Issue tracker

Issues live in GitHub Issues on tedkulp/scrapegoat (via `gh`). See `docs/agents/issue-tracker.md`.

### Triage labels

Default five-role vocabulary (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
