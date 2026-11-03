# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scrapegoat is a ROM scraper for EmulationStation that uses the ScreenScraper.fr API to gather game metadata and media files. It identifies ROMs using CRC32, MD5, and SHA1 hashes for accurate matching, handles ZIP archives transparently, and downloads media files concurrently.

## Build and Run

```bash
# Build the binary
go build -o scrapegoat ./cmd/scraper

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

**Platform Auto-Discovery**: Platforms are automatically fetched from the ScreenScraper.fr API and cached locally in `<cache-dir>/platforms.json` (default `~/.scrapegoat`). The cache expires after `--cache-hours` (default 12; `list-platforms` always uses 12 and the config's `cache_dir`, cut before any `{platform}` element since it has no `--platform`). A failed fetch falls back to an expired `platforms.json`. You can optionally define platforms manually in `config.yaml` to override API data or work offline.

## Architecture

### Package Structure

- `cmd/scraper/` - CLI entry point using Cobra (`main.go`: commands and `runScraper`; `settings.go`: `resolveScrapeSettings`; `run.go`: the per-ROM `romLoop`; `report.go`, `tally.go`, `unmatched.go`). Build it as a package (`./cmd/scraper`), not `main.go` alone.
- `pkg/config/` - Configuration loading via Viper. Handles YAML config parsing and credential validation; the artwork section is validated by `resolveScrapeSettings` only when artwork is enabled. Plain settings only (including the yaml `platforms:` block); it does not import `internal/screenscraper`.
- `internal/platform/` - The platform catalog: fetching and caching the systems list, slug and alias rules, lookup and the `list-platforms` listing.
- `internal/scanner/` - ROM file discovery. Walks directories to find files with configured extensions.
- `internal/scrape/` - The per-ROM scrape: hashes one ROM, looks it up, places its media and composes its artwork, and returns a typed scrape outcome. Prints nothing.
- `internal/screenscraper/` - ScreenScraper API client, game cache and ROM hashing.
- `internal/cutoff/` - A run's cutoff: the shared, concurrency-safe record of the first cutoff status (403, 423, 426, 430, 431), and the `*cutoff.Error` for work refused because of it. Depends on nothing else in the repo.
- `internal/media/` - Media placement: picks one file per media type, plans where each comes from (media root, media cache or ScreenScraper), and fetches and writes them with a worker pool.
- `internal/metadata/` - gamelist.xml generation for EmulationStation.
- `internal/artwork/` - Optional composed artwork (miximages) built from downloaded media, enabled by `artwork.enabled` or `--artwork`.

### Workflow (`runScraper` in cmd/scraper/main.go)

Before step 1, `resolveScrapeSettings` (`cmd/scraper/settings.go`) turns the flags and config into one `scrapeSettings` (dir defaults, `{platform}` expansion, `--auto-delete` disposition, `--artwork`, and artwork validation when enabled), and the platform catalog resolves `--platform`. The main scraping workflow follows these steps. `runScraper` scans, then hands each ROM to `scrape.Scraper.Scrape` (`internal/scrape`), which does steps 2 to 5 and prints nothing; `runScraper` prints the per-ROM lines from the result (`scrapeReport` in `cmd/scraper/report.go`), adds scraped games to the gamelist, and tallies outcomes and the unmatched ROM list in one place (`runTally` in `cmd/scraper/tally.go`). The run summary prints `Scraped`, `Not found`, `Non-game` and `Failed` counts.

1. **Scan** - `scanner.Scanner` walks the ROM directory to find files matching platform extensions
2. **Hash** - `screenscraper.HashROMFile()` computes CRC32, MD5, SHA1 for each ROM. For ZIP files, extracts and hashes the ROM inside (not the ZIP container).
3. **Lookup** - `screenscraper.Client.Lookup()` asks ScreenScraper (or the local cache) which game a ROM is, by hashes and system ID. It returns a `LookupResult` whose `Outcome` is `OutcomeMatch`, `OutcomeNotFound` or `OutcomeNonGame`; an `error` means no answer could be obtained. Retry, backoff, throttling and quota tracking live inside the client. After a cutoff it answers only from the game cache (any age) and otherwise returns a `*cutoff.Error`, which `internal/scrape` reports as `*scrape.Deferred`.
4. **Download Media** - `media.Placer.Plan` builds the ROM's media plan: one file per media type, each marked existing (already in `<media-root-dir>/<type>/`), from the media cache, or to download. `--force-redownload` marks every file to download. Dry-run reports the plan; otherwise `media.Placer.Execute` carries it out and returns a `media.Result` (placed paths plus existing / from-cache / downloaded / failed counts). A failed media file never fails the ROM.
5. **Artwork** - when enabled, `artwork.Compositor` composes images from the downloaded media
6. **Generate Metadata** - `metadata.Generator` builds gamelist.xml and writes it to `--gamelist-dir`
7. **Unmatched ROMs** - ROMs that were not found, non-game, or failed are listed, and `--auto-delete` decides whether they are deleted (unset means prompt). Deferred ROMs (lookup stopped by a cutoff, `*scrape.Deferred`) are never unmatched, so never deleted or offered. After a cutoff the gamelist carries deferred ROMs' existing entries over by `<path>` (`metadata.ReadExisting`, `Generator.CarryOver`), the summary adds `Deferred: N` and the cause, and `scrape` exits 1 after writing the gamelist. The per-ROM loop is `romLoop` in `cmd/scraper/run.go`.

### Key Design Patterns

**ZIP Handling** (`hashZippedROM` in `internal/screenscraper/hasher.go`): When hashing ZIP files, the hasher extracts the ROM file inside and hashes that content, not the ZIP archive itself. This ensures accurate matching against ScreenScraper's database which uses ROM hashes, not archive hashes.

**Region/Language Preferences** (`internal/screenscraper/types.go`): The `Game` type includes methods like `GetPreferredName()`, `GetPreferredMedia()`, and `GetPreferredSynopsis()` that accept preferred region/language lists and fall back through the list until a match is found. `screenscraper.DefaultRegions()` is the one definition of the default region order (us, wor, eu, jp); callers use it rather than writing the list out.

**ROM Scrape** (`internal/scrape/scrape.go`): `scrape.New(lookup, placer, compositor, systemID, mode, opts...)` takes a one-method `scrape.Lookup` interface (satisfied by `*screenscraper.Client`; tests use a fake), the `*media.Placer`, an optional `*artwork.Compositor` (nil disables artwork) and a `media.Mode` fixed for the run. `Scrape(rom)` returns a `scrape.Result` whose `Outcome` is `OutcomeScraped`, `OutcomeNotFound` or `OutcomeNonGame`, with `Cached`, `Hashes`, `Game`, the media `Plan`, `DryRun`, the `media.Result` and the `artwork.Result`. A hash or lookup failure is an `error` of type `*scrape.Failure` recording the `Stage` (`StageHash`, `StageLookup`) and the underlying `Err`, not a fourth outcome. A lookup stopped by a cutoff returns a `*scrape.Deferred` error instead: neither an outcome nor a failure. In dry-run a match is scraped with only the plan: nothing is written, no media is fetched and no artwork is composed. The module prints nothing; `scrape.WithProgress` reports each slow step (`StepHash`, `StepLookup`, `StepPlace`, `StepArtwork`) before it starts, which the CLI prints with `--verbose`.

**Media Placement** (`internal/media/media.go`): `media.New(cacheDir, platform, mediaRoot, workers, opts...)` with options `WithHTTPClient`, `WithCutoff` and `WithDebugOutput`; the package prints nothing else, and the CLI does the logging. `Plan` makes no requests and no writes. `Execute` fetches one ROM's files concurrently with a worker pool (`--workers`); ROMs stay sequential. Every write to the media cache and media root goes to a temp file in the same directory and is renamed into place. Each cached file has a media identity sidecar `<file>.json` holding the `systemeid`, `jeuid` and `media` query parameters of its ScreenScraper URL (never credentials or the host): a matching sidecar means the cached file is used, a different one means it is downloaded again, a missing one means the file is trusted and a sidecar written (adopted), and a URL without `jeuid` or `media` means the cached file is trusted with no sidecar. A zero-byte cached file counts as missing. The media root has an existence-only check and no sidecars. See `docs/adr/0001-media-cache-identity-sidecar.md`. `media.Types()` is the one list of media type folders; artwork layer validation (`artwork.MediaTypes`) uses it.

**API Retry Logic** (`internal/screenscraper/client.go`, `getJSONWithRetry`, used by `Lookup`, `GetSystemsList` and `GetUserInfo`): The ScreenScraper client makes up to 3 attempts per request, with exponential backoff between attempts (2s, then 4s). A rate limit response (429) also triggers a 10-second delay, skipped on the final attempt. Only failures that can clear up on their own are retried: transport errors, body-read errors, JSON decode errors (an overloaded server often returns HTML), 401 (API closed to non-members because the server is overloaded), 408 (request timeout), 429 (thread limit) and any 5xx. Every other status is permanent and returns a `*StatusError` after the first request, naming the cause: 400 (malformed request), 403 (bad developer credentials), 404, 423 (API fully closed), 426 (software name blacklisted), 430 (daily quota exceeded), 431 (failed-request quota exceeded). 403, 423, 426, 430 and 431 are cutoff statuses (`internal/cutoff`): the first one, from any request, trips the run's shared `*cutoff.Cutoff` (injected with `screenscraper.WithCutoff` and `media.WithCutoff`; each keeps its own if none is given) and returns a `*cutoff.Error` wrapping the `*StatusError`. After a cutoff, neither the client nor the media `Placer` sends any request. `Lookup` answers from a fresh game cache entry or an expired match (`GameCache.GetAny`; the file format is unchanged), and returns a `*cutoff.Error` for any other ROM, so an expired not-found or non-game entry defers the ROM instead of making it unmatched and deletable. Before a cutoff, expiry works as usual. A cutoff only ever comes from a response status, never from `Quota.Remaining()`. The platform catalog's client is not given the run's cutoff. A 404 from `jeuInfos.php` means ScreenScraper has no entry for the ROM and becomes the `OutcomeNotFound` outcome (cached, no error), as does a decoded empty "game not found" response. Status meanings are from ScreenScraper's API v2 docs, https://www.screenscraper.fr/webapi2.php.

**Lookup Client Construction** (`internal/screenscraper/client.go`): `NewClient(creds Credentials, opts ...Option)`. Options: `WithHTTPClient`, `WithClock` (one injectable clock for backoff, throttle and cache expiry), `WithCache`, `WithThrottle`, `WithCutoff`, `WithDebug`, `WithDebugOutput`. `Client.Quota()` returns the numeric quota from the latest `ssuser` response; `Client.RequestCount()` counts real HTTP requests, retries included. Debug logging prints each request URL with `devpassword` and `sspassword` replaced by `***`.

**gamelist.xml Paths** (`Generator.AddGameFromScraper` in `internal/metadata/gamelist.go`): Each entry's `<path>` is the ROM filename with a "./" prefix, as EmulationStation expects. Media tags are omitted: EmulationStation finds media by directory and ROM filename.

**Platform Caching** (`platform.Catalog` in `internal/platform/`): `platform.New(platform.Options{Source, Configured, CacheDir, Expiry, Clock, Warnings})` builds the catalog; `main.go` passes a real `screenscraper.Client` as the `Source` (anything with `GetSystemsList()`), the yaml platforms, and the cache dir and hours from `scrapeSettings`. `Lookup(slug)` checks `config.yaml` platforms first (no fetch), then loads `<cache-dir>/platforms.json`, which stores the raw systems list (ID, regional names, extensions) plus `fetched_at` and a `format` number; a missing, expired or old-format (pre-catalog slug map) file is refetched via `systemesListe.php` and rewritten. If that fetch fails and an expired `platforms.json` in the current format exists, the expired data is used and one warning line naming the fetch error goes to `Options.Warnings` (default stderr); with no usable file the lookup fails. Slugs are worked out at load, one set of rules for lookup and listing: one slug per comma-separated name part (lowercased, URL-safe via `gosimple/slug`, dashes removed); collisions go to the lowest platform ID; aliases (hard-coded in `internal/platform/aliases.go`, keyed by platform ID) beat name slugs; yaml platforms beat everything. All lookups are case-insensitive. A `Platform` has `ID`, `Name` (first name part, e.g. `Atari ST`), `Names` and `Extensions`. `List()` gives each platform its alias, else its shortest slug it actually won (ties alphabetical), so every listed slug resolves to its row.

**Caches** (all under `--cache-dir`, falling back to `output.cache_dir` from config, then `~/.scrapegoat`; `{platform}` is expanded in either): `platforms.json` and `games/` (game lookups, including not-found results; after a cutoff, expired matches still answer; expired not-found and non-game entries defer the ROM) expire after `--cache-hours`; media in `cache/<system-id>/<media-type>/` (an older `cache/<typed slug>/` folder is moved there by `media.MigrateLegacyCache` at the start of a non-dry-run scrape; see `docs/adr/0002-media-cache-keyed-by-system-id.md`) (with a `.json` media identity sidecar per file) and composed artwork in `artwork-cache/` never expire.

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
