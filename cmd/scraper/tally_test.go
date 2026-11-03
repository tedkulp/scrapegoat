package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
)

func TestRunTally_CountsEachOutcomeAndListsUnmatchedROMsInOrder(t *testing.T) {
	rom := func(name string) scanner.ROMFile { return scanner.ROMFile{Path: "/roms/" + name, Filename: name} }
	var tally runTally

	tally.record(rom("a.nes"), scrape.Result{Outcome: scrape.OutcomeScraped}, nil)
	tally.record(rom("b.nes"), scrape.Result{Outcome: scrape.OutcomeNotFound}, nil)
	tally.record(rom("c.nes"), scrape.Result{}, &scrape.Failure{Stage: scrape.StageHash, Err: errors.New("unreadable")})
	tally.record(rom("d.nes"), scrape.Result{Outcome: scrape.OutcomeNonGame}, nil)
	tally.record(rom("e.nes"), scrape.Result{Outcome: scrape.OutcomeScraped}, nil)
	tally.record(rom("f.nes"), scrape.Result{}, &scrape.Failure{Stage: scrape.StageLookup, Err: errors.New("closed")})
	tally.record(rom("g.nes"), scrape.Result{Outcome: scrape.OutcomeNotFound, Cached: true}, nil)

	if tally.scraped != 2 || tally.notFound != 2 || tally.nonGame != 1 || tally.failed != 2 {
		t.Errorf("scraped/notFound/nonGame/failed = %d/%d/%d/%d, want 2/2/1/2",
			tally.scraped, tally.notFound, tally.nonGame, tally.failed)
	}
	var got []string
	for _, r := range tally.unmatched {
		got = append(got, r.Filename)
	}
	want := []string{"b.nes", "c.nes", "d.nes", "f.nes", "g.nes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unmatched = %v, want %v", got, want)
	}
}

func TestRunTally_SummaryLinesNameTheFourCounts(t *testing.T) {
	tally := runTally{scraped: 5, notFound: 2, nonGame: 1, failed: 3}
	want := []string{"Scraped: 5", "Not found: 2", "Non-game: 1", "Failed: 3"}
	if got := tally.summaryLines(); !reflect.DeepEqual(got, want) {
		t.Errorf("summaryLines() = %v, want %v", got, want)
	}
}
