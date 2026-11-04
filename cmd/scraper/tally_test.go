package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/media"
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

func TestRunTally_SummaryLinesNameTheCounts(t *testing.T) {
	tally := runTally{scraped: 5, newMedia: 4, notFound: 2, nonGame: 1, failed: 3}
	want := []string{"Scraped: 5", "New media: 4", "Not found: 2", "Non-game: 1", "Failed: 3"}
	if got := tally.summaryLines(); !reflect.DeepEqual(got, want) {
		t.Errorf("summaryLines() = %v, want %v", got, want)
	}
}

func TestRunTally_SummaryLinesPrintZeroNewMediaAfterScrapedWithACutoff(t *testing.T) {
	tally := runTally{scraped: 1, deferred: 2, cutoff: &cutoff.Cause{}}
	got := tally.summaryLines()
	if len(got) < 2 || got[0] != "Scraped: 1" || got[1] != "New media: 0" {
		t.Errorf("summaryLines() = %v, want New media: 0 directly after Scraped", got)
	}
}

func TestRunTally_CountsScrapedROMsWithNewlyPlacedMedia(t *testing.T) {
	scraped := func(m media.Result) scrape.Result {
		return scrape.Result{Outcome: scrape.OutcomeScraped, Media: m}
	}
	cases := []struct {
		name string
		res  scrape.Result
		err  error
		want int
	}{
		{"downloaded", scraped(media.Result{Downloaded: 3, Existing: 1}), nil, 1},
		{"from cache", scraped(media.Result{FromCache: 1}), nil, 1},
		{"downloaded and from cache", scraped(media.Result{Downloaded: 2, FromCache: 2}), nil, 1},
		{"all existing", scraped(media.Result{Existing: 4}), nil, 0},
		{"all failed", scraped(media.Result{Failed: 2}), nil, 0},
		{"no media", scraped(media.Result{}), nil, 0},
		{"not found", scrape.Result{Outcome: scrape.OutcomeNotFound, Media: media.Result{Downloaded: 1}}, nil, 0},
		{"non-game", scrape.Result{Outcome: scrape.OutcomeNonGame, Media: media.Result{Downloaded: 1}}, nil, 0},
		{"failed", scraped(media.Result{Downloaded: 1}), &scrape.Failure{Stage: scrape.StageLookup, Err: errors.New("x")}, 0},
		{"deferred", scraped(media.Result{Downloaded: 1}), &scrape.Deferred{}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tally runTally
			tally.record(scanner.ROMFile{Filename: "a.nes"}, c.res, c.err)
			if tally.newMedia != c.want {
				t.Errorf("newMedia = %d, want %d", tally.newMedia, c.want)
			}
		})
	}
}

func TestRunTally_DryRunCountsPlansWithADownloadOrCacheSource(t *testing.T) {
	planned := func(sources ...media.Source) scrape.Result {
		var pl media.Plan
		for _, s := range sources {
			pl.Items = append(pl.Items, media.Item{Source: s})
		}
		return scrape.Result{Outcome: scrape.OutcomeScraped, Plan: pl, DryRun: true}
	}
	cases := []struct {
		name string
		res  scrape.Result
		want int
	}{
		{"download", planned(media.SourceExisting, media.SourceDownload), 1},
		{"cache", planned(media.SourceCache), 1},
		{"all existing", planned(media.SourceExisting, media.SourceExisting), 0},
		{"empty plan", planned(), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tally runTally
			tally.record(scanner.ROMFile{Filename: "a.nes"}, c.res, nil)
			if tally.newMedia != c.want {
				t.Errorf("newMedia = %d, want %d", tally.newMedia, c.want)
			}
		})
	}
}
