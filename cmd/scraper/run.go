package main

import (
	"errors"
	"fmt"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
	"github.com/tedkulp/scrapegoat/internal/scraper"
)

// romScraper scrapes one ROM. *scrape.Scraper satisfies it.
type romScraper interface {
	Scrape(rom scanner.ROMFile) (scrape.Result, error)
}

// romLoop scrapes a run's ROMs in order: it prints each ROM's report, tallies
// it, and adds its gamelist entry.
type romLoop struct {
	scraper romScraper
	cutoff  *cutoff.Cutoff
	// gen receives gamelist entries; nil in a dry run.
	gen *metadata.Generator
	// gamelistPath is the existing gamelist.xml that deferred ROMs' entries
	// are carried over from.
	gamelistPath string
	// quota reports the latest known quota, printed after each scraped ROM.
	quota func() scraper.Quota
	print func([]reportLine)

	existing     *metadata.Existing
	existingRead bool
}

func (l *romLoop) run(roms []scanner.ROMFile) runTally {
	var tally runTally
	for i, rom := range roms {
		l.print([]reportLine{{levelInfo, fmt.Sprintf("\n[%d/%d] Processing: %s", i+1, len(roms), rom.Filename)}})

		res, err := l.scraper.Scrape(rom)
		l.print(scrapeReport(res, err))
		tally.record(rom, res, err)
		l.addGamelistEntry(rom, res, err)

		if tally.cutoff == nil {
			if cause, ok := l.cutoff.Cause(); ok {
				tally.cutoff = &cause
				l.print([]reportLine{{levelError, fmt.Sprintf("%s: remaining ROMs deferred", cause)}})
			}
		}
	}
	return tally
}

// addGamelistEntry adds a scraped ROM's new entry, or carries a deferred
// ROM's entry over from the existing gamelist.
func (l *romLoop) addGamelistEntry(rom scanner.ROMFile, res scrape.Result, err error) {
	if l.gen == nil {
		return
	}
	var deferred *scrape.Deferred
	if errors.As(err, &deferred) {
		l.gen.CarryOver(l.existingGamelist(), rom.Path)
		return
	}
	if err == nil && res.Outcome == scrape.OutcomeScraped && !res.DryRun {
		l.gen.AddGameFromScraper(rom.Path, res.Game)
		if l.quota != nil {
			if quota := l.quota(); quota.Known {
				l.print([]reportLine{{levelInfo, fmt.Sprintf("  API Quota: %d requests remaining today", quota.Remaining())}})
			}
		}
	}
}

// existingGamelist reads the existing gamelist once, on the first deferred
// ROM. A file that cannot be read carries nothing over.
func (l *romLoop) existingGamelist() *metadata.Existing {
	if !l.existingRead {
		l.existingRead = true
		existing, err := metadata.ReadExisting(l.gamelistPath)
		if err != nil {
			l.print([]reportLine{{levelError, fmt.Sprintf("Warning: cannot carry over deferred ROMs' gamelist entries: %v", err)}})
		}
		l.existing = existing
	}
	return l.existing
}

// exitCode is the scrape command's exit status: 1 after a cutoff, else 0.
func exitCode(cut *cutoff.Cutoff) int {
	if cut.Tripped() {
		return 1
	}
	return 0
}
