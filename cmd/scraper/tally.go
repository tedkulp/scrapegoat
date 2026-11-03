package main

import (
	"fmt"

	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
)

// runTally counts a run's scrape outcomes and failures and collects its
// unmatched ROMs (not found, non-game or failed), in scan order.
type runTally struct {
	scraped   int
	notFound  int
	nonGame   int
	failed    int
	unmatched []scanner.ROMFile
}

// record adds one ROM's scrape to the tally. A non-nil err is a failure,
// whatever res says.
func (t *runTally) record(rom scanner.ROMFile, res scrape.Result, err error) {
	switch {
	case err != nil:
		t.failed++
	case res.Outcome == scrape.OutcomeScraped:
		t.scraped++
		return
	case res.Outcome == scrape.OutcomeNotFound:
		t.notFound++
	case res.Outcome == scrape.OutcomeNonGame:
		t.nonGame++
	}
	t.unmatched = append(t.unmatched, rom)
}

// summaryLines are the run summary's count lines.
func (t *runTally) summaryLines() []string {
	return []string{
		fmt.Sprintf("Scraped: %d", t.scraped),
		fmt.Sprintf("Not found: %d", t.notFound),
		fmt.Sprintf("Non-game: %d", t.nonGame),
		fmt.Sprintf("Failed: %d", t.failed),
	}
}
