package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/artwork"
	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/scrape"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

var hashes = &screenscraper.ROMHashes{CRC32: "abcd1234", MD5: "0123456789abcdef"}

func game(name string) *screenscraper.Game {
	return &screenscraper.Game{Names: []screenscraper.Name{{Region: "us", Text: name}}}
}

func TestScrapeReport_HashFailure(t *testing.T) {
	got := scrapeReport(scrape.Result{}, &scrape.Failure{Stage: scrape.StageHash, Err: errors.New("permission denied")})
	want := []reportLine{{level: levelError, text: "  Failed to hash ROM: permission denied"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScrapeReport_LookupFailure(t *testing.T) {
	got := scrapeReport(scrape.Result{Hashes: hashes}, &scrape.Failure{Stage: scrape.StageLookup, Err: errors.New("HTTP 423")})
	want := []reportLine{
		{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
		{level: levelError, text: "  Failed to get game info: HTTP 423"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScrapeReport_DeferredIsNotReportedAsAFailure(t *testing.T) {
	got := scrapeReport(scrape.Result{Hashes: hashes}, &scrape.Deferred{Cause: cutoff.Cause{StatusCode: 430}})
	want := []reportLine{
		{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
		{level: levelInfo, text: "  Deferred: ScreenScraper cut off the run; looked up again next run"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScrapeReport_NotFound(t *testing.T) {
	for cached, suffix := range map[bool]string{false: "", true: " (cached)"} {
		got := scrapeReport(scrape.Result{Outcome: scrape.OutcomeNotFound, Cached: cached, Hashes: hashes}, nil)
		want := []reportLine{
			{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
			{level: levelError, text: "  Failed to get game info: game not found in ScreenScraper database" + suffix},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("cached=%v: got %+v, want %+v", cached, got, want)
		}
	}
}

func TestScrapeReport_NonGame(t *testing.T) {
	got := scrapeReport(scrape.Result{Outcome: scrape.OutcomeNonGame, Cached: true, Hashes: hashes, Game: game("BIOS")}, nil)
	want := []reportLine{
		{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
		{level: levelInfo, text: "  Found: BIOS (cached)"},
		{level: levelInfo, text: "  Skipping: Marked as non-game by ScreenScraper"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestScrapeReport_ScrapedListsMediaFailuresSummaryAndArtwork(t *testing.T) {
	res := scrape.Result{
		Outcome: scrape.OutcomeScraped,
		Hashes:  hashes,
		Game:    game("Super Game"),
		Media: media.Result{
			Downloaded: 2, FromCache: 1, Failed: 2,
			Failures: map[string]error{"videos": errors.New("HTTP status 404"), "screenshots": errors.New("disk full")},
		},
		Artwork: artwork.Result{Paths: []string{"/m/miximages/x.png"}, Errors: []error{errors.New("artwork output 'marquee': no wheel")}},
	}
	got := scrapeReport(res, nil)
	want := []reportLine{
		{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
		{level: levelInfo, text: "  Found: Super Game"},
		{level: levelError, text: "    Failed to place screenshots: disk full"},
		{level: levelError, text: "    Failed to place videos: HTTP status 404"},
		{level: levelInfo, text: "  Media: 2 downloaded, 1 from cache, 2 failed"},
		{level: levelInfo, text: "  Warning: artwork output 'marquee': no wheel"},
		{level: levelVerbose, text: "  Generated 1 artwork files"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%+v\nwant\n%+v", got, want)
	}
}

func TestScrapeReport_DryRunShowsDownloadsCacheAndExisting(t *testing.T) {
	plan := media.Plan{Items: []media.Item{
		{MediaType: "screenshots", Source: media.SourceDownload},
		{MediaType: "wheel", Source: media.SourceDownload},
		{MediaType: "box2dfront", Source: media.SourceCache},
		{MediaType: "videos", Source: media.SourceExisting},
	}}
	got := scrapeReport(scrape.Result{Outcome: scrape.OutcomeScraped, DryRun: true, Hashes: hashes, Game: game("Super Game"), Plan: plan}, nil)
	want := []reportLine{
		{level: levelVerbose, text: "  CRC32: abcd1234, MD5: 0123456789abcdef"},
		{level: levelInfo, text: "  Found: Super Game"},
		{level: levelInfo, text: "  [DRY RUN] Would download 2 media files (1 from cache, 1 already exist)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
