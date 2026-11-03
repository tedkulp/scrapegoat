# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `scrape` skips media files that already exist in the destination, and `--dry-run` reports how many would be downloaded and how many already exist. `--force-redownload` downloads them all again.
- `user-info` shows the per-minute request limit and the failed-request quota (`Failed Requests Today: used / max`).
- An MIT `LICENSE` file.

### Changed

- Only failures that can clear up on their own are retried: network errors, timeouts, an overloaded server (401, an HTML response), 429 and 5xx. Every other error stops after one request and names the cause, such as `ScreenScraper rejected developer credentials (403)` or `ScreenScraper daily quota exceeded (430)`.
- A ROM ScreenScraper answers with 404 counts as not found and is cached like any other not-found result, so re-runs don't look it up again until `--cache-hours` passes.
- The 1-second wait applies between HTTP requests to ScreenScraper, not after every ROM, so ROMs answered from the cache no longer wait.
- The remaining-quota count shown during `scrape` also updates from the platform list request.
- `--force-redownload` also skips the media cache, so every media file is fetched from ScreenScraper again (and counts against the daily quota). `--dry-run` applies it too, and reports how many files would come from the media cache.
- A failed media download is always reported, and the `Media:` line counts it as failed. It does not fail the ROM.

### Fixed

- Game lookups retry up to 3 times again, with 2 s and 4 s backoff, instead of failing on the first error.
- A ROM cached as not found no longer crashes `scrape`; it is listed with the other unmatched ROMs.
- `user-info` no longer prints blank per-hour, per-minute and per-second limits.
- The `--verbose` cache summary shows the actual `--cache-hours` value instead of always saying 12 hours.
- A cached media file is downloaded again when a lookup now selects a different region or game for it. Each cached file gets a small `.json` sidecar recording which media it is; existing cached files without one are kept and adopted.
- An interrupted media download no longer leaves a partial file that later runs serve from the cache. An empty cached file is downloaded again.

### Security

- `--verbose` no longer prints your developer and user passwords in API URLs; both are shown as `***`.

## [1.0.0] - 2025-10-18

### Added

- `scrape` identifies ROMs by CRC32, MD5 and SHA1 against ScreenScraper.fr, hashing the ROM inside a ZIP rather than the archive.
- Media downloads (box art, 3D boxes, back covers, screenshots, title screens, wheels, videos, manuals, physical media) into EmulationStation's per-type folders, with a concurrent worker pool (`--workers`).
- `gamelist.xml` generation with names, descriptions, ratings, release dates, developers, publishers, genres and player counts.
- Platforms fetched from ScreenScraper and cached, with optional overrides in `config.yaml`, and `list-platforms` to show them.
- A cache for game lookups (including not-found results) and media, with `--cache-hours` to set when lookups and platforms expire.
- `--dry-run`, `--verbose`, `--gamelist-dir`, `--media-root-dir` and `--cache-dir`, with a `{platform}` placeholder in path flags.
- Unmatched ROMs listed at the end of a run, with a prompt to delete them, or `--auto-delete` to decide without asking.
- Optional artwork generation that composes images from downloaded media (`--artwork`, see `ARTWORK.md`).
- `user-info` for account details and API quota, and `version`.
- Release builds for Linux, macOS and Windows on x86_64 and arm64.

[Unreleased]: https://github.com/tedkulp/scrapegoat/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/tedkulp/scrapegoat/releases/tag/v1.0.0
