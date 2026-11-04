package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/metadata"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scrape"
)

// scriptedScraper answers each ROM from a script; a step may trip the cutoff,
// as the lookup client or media module would.
type scriptedScraper struct {
	steps map[string]func() (scrape.Result, error)
}

func (s *scriptedScraper) Scrape(rom scanner.ROMFile) (scrape.Result, error) {
	return s.steps[rom.Filename]()
}

func deferredBy(cut *cutoff.Cutoff) func() (scrape.Result, error) {
	return func() (scrape.Result, error) {
		cut.Trip(430, "quota")
		cause, _ := cut.Cause()
		return scrape.Result{Hashes: hashes}, &scrape.Deferred{Cause: cause}
	}
}

func returns(res scrape.Result, err error) func() (scrape.Result, error) {
	return func() (scrape.Result, error) { return res, err }
}

func TestRomLoop_CutoffDefersROMsWithoutMakingThemUnmatched(t *testing.T) {
	dir := t.TempDir()
	gamelistPath := filepath.Join(dir, "gamelist.xml")
	os.WriteFile(gamelistPath, []byte(`<gameList>
  <game><path>./d1.nes</path><name>Kept One</name><favorite>true</favorite></game>
  <game><path>./ok.nes</path><name>Stale</name></game>
</gameList>`), 0o644)

	cut := cutoff.New()
	rom := func(name string) scanner.ROMFile {
		return scanner.ROMFile{Path: filepath.Join(dir, name), Filename: name}
	}
	roms := []scanner.ROMFile{rom("ok.nes"), rom("nf.nes"), rom("d1.nes"), rom("cached-nf.nes"), rom("d2.nes")}
	s := &scriptedScraper{steps: map[string]func() (scrape.Result, error){
		"ok.nes":        returns(scrape.Result{Outcome: scrape.OutcomeScraped, Game: game("Fresh")}, nil),
		"nf.nes":        returns(scrape.Result{Outcome: scrape.OutcomeNotFound}, nil),
		"d1.nes":        deferredBy(cut),
		"cached-nf.nes": returns(scrape.Result{Outcome: scrape.OutcomeNotFound, Cached: true}, nil),
		"d2.nes":        deferredBy(cut),
	}}
	var printed []reportLine
	gen := metadata.NewGenerator()
	loop := &romLoop{scraper: s, cutoff: cut, gen: gen, gamelistPath: gamelistPath,
		report: func(lines []reportLine) { printed = append(printed, lines...) }}

	tally := loop.run(roms)

	// Unmatched ROMs from before the cutoff and from the cache are kept as
	// today; deferred ROMs are never unmatched.
	var unmatched []string
	for _, r := range tally.unmatched {
		unmatched = append(unmatched, r.Filename)
	}
	if want := []string{"nf.nes", "cached-nf.nes"}; !reflect.DeepEqual(unmatched, want) {
		t.Errorf("unmatched = %v, want %v", unmatched, want)
	}
	// With --auto-delete=true, only those are deleted.
	var removed []string
	disposeUnmatchedROMs(dispositionDelete, tally.unmatched, nil, func(p string) error {
		removed = append(removed, filepath.Base(p))
		return nil
	})
	if want := []string{"nf.nes", "cached-nf.nes"}; !reflect.DeepEqual(removed, want) {
		t.Errorf("auto-delete removed %v, want %v", removed, want)
	}

	if tally.deferred != 2 || tally.scraped != 1 || tally.notFound != 2 {
		t.Errorf("deferred/scraped/notFound = %d/%d/%d, want 2/1/2", tally.deferred, tally.scraped, tally.notFound)
	}

	// The cutoff line is printed once, naming the cause.
	const cutoffLine = "ScreenScraper daily quota exceeded (430): remaining ROMs deferred"
	n := 0
	for _, l := range printed {
		if l.text == cutoffLine {
			n++
		}
	}
	if n != 1 {
		t.Errorf("cutoff line printed %d times, want once; printed %+v", n, printed)
	}
	summary := strings.Join(tally.summaryLines(), "\n")
	for _, want := range []string{"Deferred: 2", "ScreenScraper daily quota exceeded (430)"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, summary)
		}
	}

	// The gamelist keeps d1's old entry unchanged, gets ok's new entry, and
	// has nothing for d2 (no existing entry).
	if err := gen.WriteToFile(gamelistPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(gamelistPath)
	out := string(b)
	for _, want := range []string{"<path>./d1.nes</path><name>Kept One</name><favorite>true</favorite>", "<name>Fresh</name>"} {
		if !strings.Contains(out, want) {
			t.Errorf("gamelist lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"Stale", "d2.nes", "nf.nes"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("gamelist has %q:\n%s", unwanted, out)
		}
	}

	if got := exitCode(cut); got != 1 {
		t.Errorf("exit code after cutoff = %d, want 1", got)
	}
}

func TestRomLoop_MediaCutoffIsAnnouncedWithoutDeferredROMs(t *testing.T) {
	cut := cutoff.New()
	s := &scriptedScraper{steps: map[string]func() (scrape.Result, error){
		"a.nes": func() (scrape.Result, error) {
			cut.Trip(431, "too many failures")
			return scrape.Result{Outcome: scrape.OutcomeScraped, Game: game("A"), DryRun: true}, nil
		},
	}}
	var printed []reportLine
	loop := &romLoop{scraper: s, cutoff: cut, report: func(l []reportLine) { printed = append(printed, l...) }}

	tally := loop.run([]scanner.ROMFile{{Path: "/r/a.nes", Filename: "a.nes"}})

	if tally.cutoff == nil || tally.cutoff.StatusCode != 431 {
		t.Fatalf("tally.cutoff = %+v, want 431", tally.cutoff)
	}
	if last := printed[len(printed)-1]; !strings.Contains(last.text, "(431): remaining ROMs deferred") {
		t.Errorf("last line %q, want the cutoff line", last.text)
	}
	if exitCode(cut) != 1 {
		t.Error("want exit 1")
	}
}

func TestRomLoop_RunWithoutCutoffKeepsExitZeroAndSummary(t *testing.T) {
	cut := cutoff.New()
	s := &scriptedScraper{steps: map[string]func() (scrape.Result, error){
		"a.nes": returns(scrape.Result{}, &scrape.Failure{Stage: scrape.StageLookup, Err: errors.New("HTTP 400")}),
	}}
	loop := &romLoop{scraper: s, cutoff: cut, report: func([]reportLine) {}}

	tally := loop.run([]scanner.ROMFile{{Path: "/r/a.nes", Filename: "a.nes"}})

	if exitCode(cut) != 0 {
		t.Error("want exit 0 without a cutoff")
	}
	if len(tally.unmatched) != 1 || tally.failed != 1 {
		t.Errorf("failed ROM not unmatched: %+v", tally)
	}
	if got := tally.summaryLines(); len(got) != 5 {
		t.Errorf("summary without cutoff = %v, want the five counts", got)
	}
}
