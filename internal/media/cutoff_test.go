package media

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/scraper"
)

// allTypesGame has one file of every media type, so a plan has nine items.
func allTypesGame(f *fixture) *scraper.Game {
	game := &scraper.Game{}
	for _, m := range typeMapping {
		game.Medias = append(game.Medias, scraper.Media{Type: m.screenScraper, Region: "us", Format: "png", URL: f.url("42", m.screenScraper+"(us)")})
	}
	return game
}

func TestExecute_CutoffStatusStopsEveryLaterFetch(t *testing.T) {
	f := newFixture(t)
	// Replace the server: the first request gets a 430, every other one a body.
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(430)
			io.WriteString(w, "quota")
			return
		}
		io.WriteString(w, "body")
	}))
	t.Cleanup(srv.Close)
	f.srv = srv
	game := allTypesGame(f)
	// One file is in the media cache: it is still copied after the cutoff.
	writeFile(t, f.cachePath("wheel", "Game.png"), "cached wheel")

	const workers = 3
	cut := cutoff.New()
	p := New(f.cacheDir, "nes", f.root, workers, WithHTTPClient(srv.Client()), WithCutoff(cut))
	res := p.Execute(p.Plan(game, "Game.nes", Mode{}))

	cause, ok := cut.Cause()
	if !ok || cause.StatusCode != 430 {
		t.Fatalf("cutoff = %+v, %v; want 430", cause, ok)
	}
	// Only fetches already in flight when the 430 arrived may have been sent:
	// at most one per worker.
	if got := requests.Load(); got > workers {
		t.Errorf("HTTP requests = %d, want at most %d", got, workers)
	}
	downloads := len(typeMapping) - 1
	if res.Downloaded+res.Deferred != downloads {
		t.Errorf("downloaded %d + deferred %d, want %d", res.Downloaded, res.Deferred, downloads)
	}
	if res.Deferred < downloads-workers+1 {
		t.Errorf("deferred = %d, want at least %d", res.Deferred, downloads-workers+1)
	}
	if res.Failed != 0 || len(res.Failures) != 0 {
		t.Errorf("deferred files counted as failures: %d %v", res.Failed, res.Failures)
	}
	if res.FromCache != 1 || readFile(t, f.rootPath("wheel", "Game.png")) != "cached wheel" {
		t.Errorf("media cache copy did not happen: FromCache=%d", res.FromCache)
	}
}

func TestExecute_CutoffFromLookupStopsFetchesBeforeAnyRequest(t *testing.T) {
	f := newFixture(t)
	game := allTypesGame(f)
	writeFile(t, f.cachePath("wheel", "Game.png"), "cached wheel")
	cut := cutoff.New()
	cut.Trip(403, "bad dev credentials")

	p := New(f.cacheDir, "nes", f.root, 3, WithHTTPClient(f.srv.Client()), WithCutoff(cut))
	res := p.Execute(p.Plan(game, "Game.nes", Mode{}))

	if got := f.requests.Load(); got != 0 {
		t.Errorf("HTTP requests = %d, want 0", got)
	}
	if res.Deferred != len(typeMapping)-1 || res.FromCache != 1 || res.Failed != 0 {
		t.Errorf("deferred=%d fromCache=%d failed=%d, want %d/1/0", res.Deferred, res.FromCache, res.Failed, len(typeMapping)-1)
	}
	if _, ok := res.Placed["wheel"]; !ok {
		t.Error("cached wheel not placed")
	}
}

func TestExecute_NonCutoffStatusIsAPerFileFailure(t *testing.T) {
	f := newFixture(t) // serves 404 for unknown media
	cut := cutoff.New()
	p := New(f.cacheDir, "nes", f.root, 1, WithHTTPClient(f.srv.Client()), WithCutoff(cut))
	res := p.Execute(p.Plan(ssGame(f.url("42", "ss(us)")), "Game.nes", Mode{}))

	if cut.Tripped() || res.Failed != 1 || res.Deferred != 0 {
		t.Errorf("tripped=%v failed=%d deferred=%d, want false/1/0", cut.Tripped(), res.Failed, res.Deferred)
	}
	var ce *cutoff.Error
	if errors.As(res.Failures["screenshots"], &ce) {
		t.Errorf("404 failure is a cutoff error: %v", res.Failures["screenshots"])
	}
}
