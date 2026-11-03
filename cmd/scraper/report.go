package main

import (
	"errors"
	"fmt"

	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/scrape"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

type reportLevel int

const (
	levelInfo    reportLevel = iota // stdout
	levelVerbose                    // stdout, only with --verbose
	levelError                      // stderr
)

type reportLine struct {
	level reportLevel
	text  string
}

// scrapeReport is the per-ROM log for one scrape: what Scrape returned,
// turned into the lines runScraper prints.
func scrapeReport(res scrape.Result, err error) []reportLine {
	var lines []reportLine
	add := func(level reportLevel, format string, args ...any) {
		lines = append(lines, reportLine{level, fmt.Sprintf(format, args...)})
	}

	if res.Hashes != nil {
		add(levelVerbose, "  CRC32: %s, MD5: %s", res.Hashes.CRC32, res.Hashes.MD5)
	}

	cached := ""
	if res.Cached {
		cached = " (cached)"
	}

	if err != nil {
		var f *scrape.Failure
		var d *scrape.Deferred
		switch {
		case errors.As(err, &d):
			add(levelInfo, "  Deferred: ScreenScraper cut off the run; looked up again next run")
		case !errors.As(err, &f):
			add(levelError, "  Failed to get game info: %v", err)
		case f.Stage == scrape.StageHash:
			add(levelError, "  Failed to hash ROM: %v", f.Err)
		default:
			add(levelError, "  Failed to get game info: %v", f.Err)
		}
		return lines
	}

	if res.Outcome == scrape.OutcomeNotFound {
		add(levelError, "  Failed to get game info: game not found in ScreenScraper database%s", cached)
		return lines
	}

	add(levelInfo, "  Found: %s%s", res.Game.GetPreferredName(screenscraper.DefaultRegions()), cached)

	if res.Outcome == scrape.OutcomeNonGame {
		add(levelInfo, "  Skipping: Marked as non-game by ScreenScraper")
		return lines
	}

	if res.DryRun {
		add(levelInfo, "  [DRY RUN] Would download %d media files (%d from cache, %d already exist)",
			res.Plan.Count(media.SourceDownload), res.Plan.Count(media.SourceCache), res.Plan.Count(media.SourceExisting))
		return lines
	}

	for _, mediaType := range media.Types() {
		if err, ok := res.Media.Failures[mediaType]; ok {
			add(levelError, "    Failed to place %s: %v", mediaType, err)
		}
	}
	if summary := mediaSummary(res.Media); summary != "" {
		add(levelInfo, "  Media: %s", summary)
	}
	for _, err := range res.Artwork.Errors {
		add(levelInfo, "  Warning: %v", err)
	}
	if len(res.Artwork.Paths) > 0 {
		add(levelVerbose, "  Generated %d artwork files", len(res.Artwork.Paths))
	}
	return lines
}

// printReport prints report lines through the run's loggers.
func printReport(lines []reportLine) {
	for _, l := range lines {
		switch l.level {
		case levelError:
			logError("%s", l.text)
		case levelVerbose:
			logVerbose("%s", l.text)
		default:
			logInfo("%s", l.text)
		}
	}
}
