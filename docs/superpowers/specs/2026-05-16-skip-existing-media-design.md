# Design: Skip Existing Media Files on Scrape

**Date:** 2026-05-16  
**Status:** Approved

## Problem

When moving to a new machine or regenerating a lost `gamelist.xml`, users have their ROM files and final media directories intact but no `gamelist.xml`. Re-running `scrape` re-downloads all media files even though they already exist in the final destination directories. The intermediate cache (`~/.scrapegoat/cache/`) is checked, but the final media directories (`{mediaRootDir}/{type}/`) are not.

## Goal

Make `scrape` skip downloading media files that already exist in their final destination, with an opt-out flag for users who want to force re-download.

## Design

### New Flag

Add `--force-redownload` (bool, default `false`) to `scrapeCmd`.

When `false` (default): check final media directories before downloading.  
When `true`: skip the check and download everything, overwriting existing files.

### Flow Change in `runScraper`

After `buildMediaFileList()` returns the list of media files to fetch, and before passing that list to `dl.Download()`, add a pre-check:

```
for each mediaFile in mediaFiles:
    finalPath = filepath.Join(mediaRootDir, mediaFile.Type, mediaFile.Filename)
    if file exists at finalPath AND NOT forceRedownload:
        add finalPath to downloadedFiles[mediaFile.Type]
        remove mediaFile from download list
        increment existingCount
```

The remaining files (not found in final location) are passed to `dl.Download()` as before. The intermediate cache check inside the downloader still applies to those remaining files.

### Logging

Update the media summary line to include an "existing" count:

- Before: `Media: 2 downloaded, 5 from cache`
- After (with existing files): `Media: 3 existing, 1 downloaded, 3 from cache`

Only show non-zero counts (same pattern as current behavior).

### Files Changed

- `cmd/scraper/main.go`
  - `init()`: register `--force-redownload` flag on `scrapeCmd`
  - `runScraper()`: add pre-check loop before `dl.Download()`, update media summary logging

No other files need modification. The downloader, metadata generator, and all other packages are unchanged.

## Out of Scope

- Priming the intermediate cache from final-location files (adds complexity, minimal benefit after first run)
- A separate `generate-gamelist` command (this approach makes it unnecessary)
- Checking artwork output files for the same skip logic (artwork is derived from media, regenerating it is fast)
