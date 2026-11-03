package main

import (
	"errors"
	"fmt"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
)

// runTally counts a run's scrape outcomes, failures and deferred ROMs, and
// collects its unmatched ROMs (not found, non-game or failed), in scan order.
// Deferred ROMs are never unmatched.
type runTally struct {
	scraped   int
	notFound  int
	nonGame   int
	failed    int
	deferred  int
	unmatched []scanner.ROMFile
	// cutoff is the run's cutoff, once there has been one.
	cutoff *cutoff.Cause
}

// record adds one ROM's scrape to the tally. A non-nil err is a failure,
// whatever res says, unless it is a *scrape.Deferred.
func (t *runTally) record(rom scanner.ROMFile, res scrape.Result, err error) {
	var deferred *scrape.Deferred
	switch {
	case errors.As(err, &deferred):
		t.deferred++
		return
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

// summaryLines are the run summary's count lines. A run with a cutoff adds
// its deferred count and the cause.
func (t *runTally) summaryLines() []string {
	lines := []string{
		fmt.Sprintf("Scraped: %d", t.scraped),
		fmt.Sprintf("Not found: %d", t.notFound),
		fmt.Sprintf("Non-game: %d", t.nonGame),
		fmt.Sprintf("Failed: %d", t.failed),
	}
	if t.cutoff != nil {
		lines = append(lines,
			fmt.Sprintf("Deferred: %d", t.deferred),
			fmt.Sprintf("Cutoff: %s; deferred ROMs were kept and are looked up again next run", t.cutoff))
	}
	return lines
}
