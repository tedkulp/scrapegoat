# Skip Existing Media Files Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skip downloading media files that already exist in their final destination directories, with a `--force-redownload` flag to override.

**Architecture:** Extract a pure helper function `filterExistingMediaFiles` that splits the media file list into "already on disk" and "needs download", then wire it into `runScraper` before calling `dl.Download()`. The flag bypasses the check entirely.

**Tech Stack:** Go standard library (`os.Stat`, `filepath.Join`), Cobra for the new flag.

---

### Task 1: Add `--force-redownload` flag

**Files:**
- Modify: `cmd/scraper/main.go:68-107` (var block and `init()`)

- [ ] **Step 1: Add the flag variable**

In `cmd/scraper/main.go`, add `forceRedownload` to the existing var block (around line 68):

```go
var (
	// Scrape command flags
	platformName    string
	romDir          string
	gamelistDir     string
	mediaRootDir    string
	cacheDir        string
	workers         int
	dryRun          bool
	autoDelete      string
	artworkFlag     string
	cacheHours      int
	forceRedownload bool // new
)
```

- [ ] **Step 2: Register the flag in `init()`**

Add this line in `init()` alongside the other `scrapeCmd.Flags()` calls:

```go
scrapeCmd.Flags().BoolVar(&forceRedownload, "force-redownload", false, "re-download media files even if they already exist in the destination directory")
```

- [ ] **Step 3: Build and verify the flag is visible**

```bash
go build -o scrapegoat cmd/scraper/main.go && ./scrapegoat scrape --help | grep force-redownload
```

Expected output contains:
```
--force-redownload   re-download media files even if they already exist in the destination directory
```

- [ ] **Step 4: Commit**

```bash
git add cmd/scraper/main.go
git commit -m "feat: add --force-redownload flag to scrape command"
```

---

### Task 2: Extract and test `filterExistingMediaFiles`

**Files:**
- Modify: `cmd/scraper/main.go` (add helper function near `buildMediaFileList`)
- Create: `cmd/scraper/main_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/scraper/main_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/downloader"
)

func TestFilterExistingMediaFiles_AllMissing(t *testing.T) {
	dir := t.TempDir()

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 0 {
		t.Errorf("expected 0 existing, got %d", len(existing))
	}
	if len(toDownload) != 2 {
		t.Errorf("expected 2 to download, got %d", len(toDownload))
	}
}

func TestFilterExistingMediaFiles_SomeExist(t *testing.T) {
	dir := t.TempDir()

	// Create the wheel file in its final location
	wheelDir := filepath.Join(dir, "wheel")
	if err := os.MkdirAll(wheelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wheelFile := filepath.Join(wheelDir, "game.png")
	if err := os.WriteFile(wheelFile, []byte("fake image"), 0o644); err != nil {
		t.Fatal(err)
	}

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 1 {
		t.Errorf("expected 1 existing, got %d", len(existing))
	}
	if existing["wheel"] != wheelFile {
		t.Errorf("expected existing[wheel] = %q, got %q", wheelFile, existing["wheel"])
	}
	if len(toDownload) != 1 {
		t.Errorf("expected 1 to download, got %d", len(toDownload))
	}
	if toDownload[0].Type != "screenshots" {
		t.Errorf("expected toDownload[0].Type = screenshots, got %s", toDownload[0].Type)
	}
}

func TestFilterExistingMediaFiles_AllExist(t *testing.T) {
	dir := t.TempDir()

	for _, mediaType := range []string{"wheel", "screenshots"} {
		typeDir := filepath.Join(dir, mediaType)
		if err := os.MkdirAll(typeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(typeDir, "game.png"), []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 2 {
		t.Errorf("expected 2 existing, got %d", len(existing))
	}
	if len(toDownload) != 0 {
		t.Errorf("expected 0 to download, got %d", len(toDownload))
	}
}

func TestFilterExistingMediaFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	existing, toDownload := filterExistingMediaFiles(nil, dir)
	if len(existing) != 0 {
		t.Errorf("expected 0 existing, got %d", len(existing))
	}
	if len(toDownload) != 0 {
		t.Errorf("expected 0 to download, got %d", len(toDownload))
	}
}
```

- [ ] **Step 2: Run the test — verify it fails**

```bash
go test ./cmd/scraper/ -run TestFilterExistingMediaFiles -v
```

Expected: compile error — `filterExistingMediaFiles undefined`

- [ ] **Step 3: Add `filterExistingMediaFiles` to `main.go`**

Add this function in `cmd/scraper/main.go` near `buildMediaFileList` (around line 587):

```go
// filterExistingMediaFiles splits mediaFiles into those already present in the
// final destination and those that still need to be downloaded.
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
```

- [ ] **Step 4: Run the tests — verify they pass**

```bash
go test ./cmd/scraper/ -run TestFilterExistingMediaFiles -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/scraper/main.go cmd/scraper/main_test.go
git commit -m "feat: add filterExistingMediaFiles helper with tests"
```

---

### Task 3: Wire `filterExistingMediaFiles` into `runScraper` and update logging

**Files:**
- Modify: `cmd/scraper/main.go` — `runScraper` function (around lines 394-438)

The current download block looks like this (starting around line 394):

```go
mediaFiles := buildMediaFileList(game, rom.Filename)
logVerbose("  Downloading %d media files...", len(mediaFiles))

downloadedFiles := make(map[string]string)
mediaAPICallCount := 0
if len(mediaFiles) > 0 {
    results := dl.Download(mediaFiles)
    cachedCount := 0
    downloadedCount := 0

    for _, result := range results {
        // ...collect results, copy to final...
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
```

- [ ] **Step 1: Replace the download block**

Replace the block above with this updated version:

```go
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
```

- [ ] **Step 2: Build and verify no compile errors**

```bash
go build -o scrapegoat cmd/scraper/main.go
```

Expected: no output (success).

- [ ] **Step 3: Run all tests**

```bash
go test ./cmd/scraper/ -v
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/scraper/main.go
git commit -m "feat: skip media files already present in final destination

Default behavior now checks final media directories before downloading.
Use --force-redownload to override and re-download all files."
```

---

### Task 4: Smoke test

- [ ] **Step 1: Verify `--help` shows the new flag**

```bash
./scrapegoat scrape --help
```

Expected: output includes `--force-redownload`.

- [ ] **Step 2: Dry-run against a real ROM directory (if available)**

```bash
./scrapegoat scrape --platform nes --rom-dir /path/to/roms --dry-run --verbose
```

Expected: no errors, normal dry-run output.

- [ ] **Step 3: Build final binary**

```bash
go build -o scrapegoat cmd/scraper/main.go
```

Expected: success.
