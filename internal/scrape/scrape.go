// Package scrape turns one ROM into EmulationStation metadata and media: it
// hashes the ROM, looks it up, places its media and composes its artwork,
// and reports what happened as a scrape outcome. It prints nothing.
package scrape

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tedkulp/scrapegoat/internal/artwork"
	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/scraper"
)

// Lookup asks which game a ROM is. *scraper.Client satisfies it.
type Lookup interface {
	Lookup(systemID int, romName string, hashes *scraper.ROMHashes) (scraper.LookupResult, error)
}

// Outcome is what a scrape produced: scraped, not found, or non-game.
type Outcome int

const (
	// OutcomeScraped: the ROM matched a game and its media and artwork were
	// placed (or, in a dry run, planned).
	OutcomeScraped Outcome = iota
	// OutcomeNotFound: ScreenScraper has no entry for the ROM.
	OutcomeNotFound
	// OutcomeNonGame: ScreenScraper classifies the ROM as something other
	// than a game.
	OutcomeNonGame
)

// String returns the glossary name of the outcome.
func (o Outcome) String() string {
	switch o {
	case OutcomeScraped:
		return "scraped"
	case OutcomeNotFound:
		return "not found"
	case OutcomeNonGame:
		return "non-game"
	}
	return fmt.Sprintf("Outcome(%d)", int(o))
}

// Stage is the step of a scrape that failed.
type Stage int

const (
	// StageHash: the ROM could not be hashed.
	StageHash Stage = iota
	// StageLookup: the lookup got no answer.
	StageLookup
)

// String names the stage.
func (s Stage) String() string {
	switch s {
	case StageHash:
		return "hash"
	case StageLookup:
		return "lookup"
	}
	return fmt.Sprintf("Stage(%d)", int(s))
}

// Failure is the error Scrape returns when it could not hash or look up the
// ROM. A failure is not a scrape outcome.
type Failure struct {
	Stage Stage
	Err   error
}

func (f *Failure) Error() string { return fmt.Sprintf("%s: %v", f.Stage, f.Err) }

// Unwrap returns the underlying error.
func (f *Failure) Unwrap() error { return f.Err }

// Deferred is the error Scrape returns when a cutoff stopped the lookup. The
// ROM is a deferred ROM: neither a scrape outcome nor a failure, and never
// unmatched. A later run looks it up again.
type Deferred struct {
	Cause cutoff.Cause
}

func (d *Deferred) Error() string { return "deferred: " + d.Cause.String() }

// Result is what scraping one ROM produced.
type Result struct {
	Outcome Outcome
	// Cached reports whether the lookup answer came from the local cache.
	Cached bool
	// Hashes are the ROM's hashes.
	Hashes *scraper.ROMHashes
	// Game is set for scraped and non-game.
	Game *scraper.Game
	// Plan is the media plan (scraped only).
	Plan media.Plan
	// DryRun is true when the plan was only built, not carried out.
	DryRun bool
	// Media is what placing the plan produced; empty in a dry run.
	Media media.Result
	// Artwork is what composing artwork produced; empty in a dry run or
	// without a compositor.
	Artwork artwork.Result
}

// Scraper scrapes ROMs of one platform.
type Scraper struct {
	lookup     Lookup
	placer     *media.Placer
	compositor *artwork.Compositor
	systemID   int
	mode       media.Mode
}

// New returns a Scraper. compositor may be nil, which disables artwork.
func New(lookup Lookup, placer *media.Placer, compositor *artwork.Compositor, systemID int, mode media.Mode) *Scraper {
	return &Scraper{lookup: lookup, placer: placer, compositor: compositor, systemID: systemID, mode: mode}
}

// Scrape scrapes one ROM.
func (s *Scraper) Scrape(rom scanner.ROMFile) (Result, error) {
	hashes, err := scraper.HashROMFile(rom.Path)
	if err != nil {
		return Result{}, &Failure{Stage: StageHash, Err: err}
	}
	answer, err := s.lookup.Lookup(s.systemID, rom.Filename, hashes)
	if err != nil {
		var ce *cutoff.Error
		if errors.As(err, &ce) {
			return Result{Hashes: hashes}, &Deferred{Cause: ce.Cause}
		}
		return Result{Hashes: hashes}, &Failure{Stage: StageLookup, Err: err}
	}
	res := Result{Hashes: hashes, Game: answer.Game, Cached: answer.Cached}
	switch answer.Outcome {
	case scraper.OutcomeNotFound:
		res.Outcome = OutcomeNotFound
		return res, nil
	case scraper.OutcomeNonGame:
		res.Outcome = OutcomeNonGame
		return res, nil
	}
	res.Outcome = OutcomeScraped
	res.Plan = s.placer.Plan(answer.Game, rom.Filename, s.mode)
	if s.mode.DryRun {
		res.DryRun = true
		return res, nil
	}
	res.Media = s.placer.Execute(res.Plan)
	if s.compositor != nil && len(res.Media.Placed) > 0 {
		romName := strings.TrimSuffix(rom.Filename, filepath.Ext(rom.Filename))
		res.Artwork = s.compositor.GenerateArtwork(romName, artwork.MediaFiles(res.Media.Placed), s.placer.Root())
	}
	return res, nil
}
