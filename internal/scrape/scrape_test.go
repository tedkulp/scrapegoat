package scrape

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/artwork"
	"github.com/tedkulp/scrapegoat/internal/cutoff"
	"github.com/tedkulp/scrapegoat/internal/media"
	"github.com/tedkulp/scrapegoat/internal/scanner"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

const systemID = 3

// fakeLookup answers every lookup with a fixed result and error, and records
// what it was asked.
type fakeLookup struct {
	result   screenscraper.LookupResult
	err      error
	calls    int
	systemID int
	romName  string
	hashes   *screenscraper.ROMHashes
}

func (f *fakeLookup) Lookup(systemID int, romName string, hashes *screenscraper.ROMHashes) (screenscraper.LookupResult, error) {
	f.calls++
	f.systemID, f.romName, f.hashes = systemID, romName, hashes
	return f.result, f.err
}

// env is a ROM folder, media root, cache dir and an httptest media server
// that serves a small PNG for every request.
type env struct {
	t        *testing.T
	romDir   string
	root     string
	cacheDir string
	srv      *httptest.Server
	requests atomic.Int32
}

func newEnv(t *testing.T) *env {
	e := &env{t: t, romDir: t.TempDir(), root: t.TempDir(), cacheDir: t.TempDir()}
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.requests.Add(1)
		img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		png.Encode(w, img)
	}))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *env) rom(name, content string) scanner.ROMFile {
	e.t.Helper()
	path := filepath.Join(e.romDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
	return scanner.ROMFile{Path: path, Filename: name, Size: int64(len(content))}
}

func (e *env) placer() *media.Placer {
	return media.New(e.cacheDir, 3, e.root, 2, media.WithHTTPClient(e.srv.Client()))
}

func (e *env) compositor() *artwork.Compositor {
	return artwork.NewCompositor(artwork.Config{
		OutputDir: "miximages",
		Outputs: []artwork.OutputDefinition{{
			Type: "mix", Width: 8, Height: 8, Format: "png",
			Layers: []artwork.LayerDefinition{{Resource: "screenshots"}},
		}},
	}, e.cacheDir)
}

func (e *env) mediaURL(m string) string {
	return e.srv.URL + "/api2/mediaJeu.php?devid=d&devpassword=p&systemeid=3&jeuid=42&media=" + m
}

func (e *env) game() *screenscraper.Game {
	return &screenscraper.Game{
		ID:    "42",
		Names: []screenscraper.Name{{Region: "us", Text: "Super Game"}},
		Medias: []screenscraper.Media{
			{Type: "ss", Region: "us", Format: "png", URL: e.mediaURL("ss(us)")},
			{Type: "box-2D", Region: "us", Format: "png", URL: e.mediaURL("box-2D(us)")},
		},
	}
}

// files lists every regular file under dir, relative to it.
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, rel)
		}
		return nil
	})
	return out
}

func TestScrape_MatchIsScrapedWithMediaPlacedAndArtworkComposed(t *testing.T) {
	e := newEnv(t)
	game := e.game()
	lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeMatch, Game: game}}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

	rom := e.rom("Super Game (USA).nes", "rom bytes")
	res, err := s.Scrape(rom)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if res.Outcome != OutcomeScraped {
		t.Fatalf("Outcome = %v, want scraped", res.Outcome)
	}
	if res.Game != game {
		t.Errorf("Game = %v, want the lookup's game", res.Game)
	}
	if lookup.systemID != systemID || lookup.romName != rom.Filename || lookup.hashes == nil || lookup.hashes.CRC32 == "" {
		t.Errorf("lookup got system %d, name %q, hashes %+v", lookup.systemID, lookup.romName, lookup.hashes)
	}
	if res.Media.Downloaded != 2 {
		t.Errorf("Media.Downloaded = %d, want 2", res.Media.Downloaded)
	}
	wantShot := filepath.Join(e.root, "screenshots", "Super Game (USA).png")
	if res.Media.Placed["screenshots"] != wantShot {
		t.Errorf("Placed[screenshots] = %q, want %q", res.Media.Placed["screenshots"], wantShot)
	}
	if _, err := os.Stat(wantShot); err != nil {
		t.Errorf("screenshot not placed: %v", err)
	}
	wantArt := filepath.Join(e.root, "miximages", "Super Game (USA).png")
	if len(res.Artwork.Paths) != 1 || res.Artwork.Paths[0] != wantArt {
		t.Errorf("Artwork.Paths = %v (errors %v), want [%s]", res.Artwork.Paths, res.Artwork.Errors, wantArt)
	}
	if _, err := os.Stat(wantArt); err != nil {
		t.Errorf("artwork not written: %v", err)
	}
}

func TestScrape_DryRunReturnsScrapedWithPlanAndWritesNothing(t *testing.T) {
	e := newEnv(t)
	// One file already in the media cache, one to download.
	cached := filepath.Join(e.cacheDir, "cache", "3", "box2dfront", "Super Game (USA).png")
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, []byte("box"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheBefore := files(t, e.cacheDir)

	lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeMatch, Game: e.game()}}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{DryRun: true})

	res, err := s.Scrape(e.rom("Super Game (USA).nes", "rom bytes"))
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if res.Outcome != OutcomeScraped || !res.DryRun {
		t.Fatalf("Outcome = %v, DryRun = %v; want scraped, dry run", res.Outcome, res.DryRun)
	}
	if got := res.Plan.Count(media.SourceDownload); got != 1 {
		t.Errorf("plan downloads %d files, want 1", got)
	}
	if got := res.Plan.Count(media.SourceCache); got != 1 {
		t.Errorf("plan takes %d files from the media cache, want 1", got)
	}
	if got := files(t, e.root); len(got) != 0 {
		t.Errorf("media root has %v, want nothing", got)
	}
	if got := files(t, e.cacheDir); len(got) != len(cacheBefore) {
		t.Errorf("cache dir has %v, want only %v", got, cacheBefore)
	}
	if n := e.requests.Load(); n != 0 {
		t.Errorf("%d media requests, want 0", n)
	}
	if len(res.Artwork.Paths) != 0 || len(res.Artwork.Errors) != 0 {
		t.Errorf("Artwork = %+v, want none composed", res.Artwork)
	}
}

func TestScrape_DryRunWithForcePlansLikeARealRunWithForce(t *testing.T) {
	plans := map[bool][]media.Item{}
	for _, dry := range []bool{true, false} {
		e := newEnv(t)
		// Every file is already in the media root; force ignores that.
		for _, dir := range []string{"screenshots", "box2dfront"} {
			p := filepath.Join(e.root, dir, "Super Game (USA).png")
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte("old"), 0o644)
		}
		lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeMatch, Game: e.game()}}
		s := New(lookup, e.placer(), nil, systemID, media.Mode{DryRun: dry, Force: true})
		res, err := s.Scrape(e.rom("Super Game (USA).nes", "rom bytes"))
		if err != nil {
			t.Fatalf("dry=%v: Scrape: %v", dry, err)
		}
		var got []media.Item
		for _, it := range res.Plan.Items {
			// Paths differ between the two temp dirs; compare what the plan decided.
			got = append(got, media.Item{MediaType: it.MediaType, Source: it.Source, Path: filepath.Base(it.Path)})
		}
		plans[dry] = got
	}
	if !reflect.DeepEqual(plans[true], plans[false]) {
		t.Errorf("dry-run plan %+v, real plan %+v; want equal", plans[true], plans[false])
	}
	for _, it := range plans[true] {
		if it.Source != media.SourceDownload {
			t.Errorf("%s: Source %v, want download under force", it.MediaType, it.Source)
		}
	}
	if len(plans[true]) != 2 {
		t.Errorf("plan has %d items, want 2", len(plans[true]))
	}
}

func TestScrape_NotFoundReportsWhetherCached(t *testing.T) {
	for _, cached := range []bool{false, true} {
		e := newEnv(t)
		lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeNotFound, Cached: cached}}
		s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

		res, err := s.Scrape(e.rom("Unknown.nes", "mystery"))
		if err != nil {
			t.Fatalf("cached=%v: Scrape: %v", cached, err)
		}
		if res.Outcome != OutcomeNotFound {
			t.Errorf("cached=%v: Outcome = %v, want not found", cached, res.Outcome)
		}
		if res.Cached != cached {
			t.Errorf("Cached = %v, want %v", res.Cached, cached)
		}
		if got := files(t, e.root); len(got) != 0 {
			t.Errorf("cached=%v: media root has %v, want nothing", cached, got)
		}
		if n := e.requests.Load(); n != 0 {
			t.Errorf("cached=%v: %d media requests, want 0", cached, n)
		}
	}
}

func TestScrape_HashFailureRecordsHashStage(t *testing.T) {
	e := newEnv(t)
	lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeMatch, Game: e.game()}}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

	missing := scanner.ROMFile{Path: filepath.Join(e.romDir, "gone.nes"), Filename: "gone.nes"}
	_, err := s.Scrape(missing)

	var ferr *Failure
	if !errors.As(err, &ferr) {
		t.Fatalf("err = %v, want a *Failure", err)
	}
	if ferr.Stage != StageHash {
		t.Errorf("Stage = %v, want hash", ferr.Stage)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want it to wrap the missing-file error", err)
	}
	if lookup.calls != 0 {
		t.Errorf("lookup called %d times, want 0", lookup.calls)
	}
}

func TestScrape_LookupStoppedByCutoffIsDeferredNotAFailure(t *testing.T) {
	e := newEnv(t)
	cause := cutoff.Cause{StatusCode: 430, Body: "quota"}
	lookup := &fakeLookup{err: fmt.Errorf("failed to look up game: %w", &cutoff.Error{Cause: cause})}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

	res, err := s.Scrape(e.rom("Game.nes", "game"))

	var ferr *Failure
	if errors.As(err, &ferr) {
		t.Fatalf("err = %v, a cutoff must not be a *Failure", err)
	}
	var d *Deferred
	if !errors.As(err, &d) {
		t.Fatalf("err = %v, want a *Deferred", err)
	}
	if d.Cause != cause {
		t.Errorf("Deferred cause = %+v, want %+v", d.Cause, cause)
	}
	if res.Hashes == nil {
		t.Error("deferred result has no hashes; hashing must still run")
	}
	if e.requests.Load() != 0 || len(files(t, e.root)) != 0 {
		t.Error("a deferred ROM must place no media")
	}
}

func TestScrape_LookupFailureRecordsLookupStageAndError(t *testing.T) {
	e := newEnv(t)
	cause := errors.New("ScreenScraper is closed")
	lookup := &fakeLookup{err: cause}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

	_, err := s.Scrape(e.rom("Game.nes", "game"))

	var ferr *Failure
	if !errors.As(err, &ferr) {
		t.Fatalf("err = %v, want a *Failure", err)
	}
	if ferr.Stage != StageLookup || ferr.Err != cause {
		t.Errorf("got stage %v, err %v; want lookup, %v", ferr.Stage, ferr.Err, cause)
	}
	if got := files(t, e.root); len(got) != 0 {
		t.Errorf("media root has %v, want nothing", got)
	}
}

func TestScrape_NonGameCarriesTheGameAndPlacesNothing(t *testing.T) {
	e := newEnv(t)
	game := e.game()
	game.Names = []screenscraper.Name{{Region: "wor", Text: "ZZZ(notgame):#NONGAME"}}
	lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: screenscraper.OutcomeNonGame, Game: game, Cached: true}}
	s := New(lookup, e.placer(), e.compositor(), systemID, media.Mode{})

	res, err := s.Scrape(e.rom("BIOS.nes", "bios"))
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	if res.Outcome != OutcomeNonGame || res.Game != game || !res.Cached {
		t.Errorf("got outcome %v, game %v, cached %v; want non-game, the lookup's game, cached", res.Outcome, res.Game, res.Cached)
	}
	if got := files(t, e.root); len(got) != 0 {
		t.Errorf("media root has %v, want nothing", got)
	}
	if n := e.requests.Load(); n != 0 {
		t.Errorf("%d media requests, want 0", n)
	}
}

func TestScrape_ReportsProgressBeforeEachSlowStep(t *testing.T) {
	cases := []struct {
		name    string
		outcome screenscraper.Outcome
		art     bool
		mode    media.Mode
		want    []Step
	}{
		{"scraped with artwork", screenscraper.OutcomeMatch, true, media.Mode{}, []Step{StepHash, StepLookup, StepPlace, StepArtwork}},
		{"scraped without artwork", screenscraper.OutcomeMatch, false, media.Mode{}, []Step{StepHash, StepLookup, StepPlace}},
		{"dry run", screenscraper.OutcomeMatch, true, media.Mode{DryRun: true}, []Step{StepHash, StepLookup}},
		{"not found", screenscraper.OutcomeNotFound, true, media.Mode{}, []Step{StepHash, StepLookup}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			lookup := &fakeLookup{result: screenscraper.LookupResult{Outcome: tc.outcome, Game: e.game()}}
			var compositor *artwork.Compositor
			if tc.art {
				compositor = e.compositor()
			}
			var got []Step
			var placing *media.Plan
			s := New(lookup, e.placer(), compositor, systemID, tc.mode, WithProgress(func(step Step, plan *media.Plan) {
				got = append(got, step)
				if step == StepPlace {
					placing = plan
				}
			}))

			res, err := s.Scrape(e.rom("Super Game (USA).nes", "rom bytes"))
			if err != nil {
				t.Fatalf("Scrape: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("steps = %v, want %v", got, tc.want)
			}
			if placing != nil && len(placing.Items) != len(res.Plan.Items) {
				t.Errorf("StepPlace got a plan of %d items, want %d", len(placing.Items), len(res.Plan.Items))
			}
		})
	}
}
