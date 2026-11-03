package platform

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

type fakeSource struct {
	systems []scraper.System
	err     error
	calls   int
}

func (f *fakeSource) GetSystemsList() ([]scraper.System, error) {
	f.calls++
	return f.systems, f.err
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

var t0 = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func system(id int, name string, exts ...string) scraper.System {
	return scraper.System{ID: id, Names: []scraper.Name{{Region: "us", Text: name}}, Extensions: exts}
}

func newCatalog(src SystemsSource, dir string, clock Clock, configured map[string]Platform) *Catalog {
	return New(Options{
		Source:     src,
		Configured: configured,
		CacheDir:   dir,
		Expiry:     12 * time.Hour,
		Clock:      clock,
	})
}

func mustLookup(t *testing.T, c *Catalog, slug string) Platform {
	t.Helper()
	p, err := c.Lookup(slug)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", slug, err)
	}
	return p
}

func TestLookup_FetchesAndExposesFirstNamePart(t *testing.T) {
	src := &fakeSource{systems: []scraper.System{
		system(42, "Atari ST,Atari STE,520 ST", ".st", ".msa"),
	}}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, nil)

	p := mustLookup(t, c, "atarist")
	if p.ID != 42 || p.Name != "Atari ST" {
		t.Fatalf("got %+v", p)
	}
	if want := []string{"Atari ST", "Atari STE", "520 ST"}; strings.Join(p.Names, "|") != strings.Join(want, "|") {
		t.Fatalf("Names = %q, want %q", p.Names, want)
	}
	if strings.Join(p.Extensions, ",") != ".st,.msa" {
		t.Fatalf("Extensions = %q", p.Extensions)
	}
	for _, s := range []string{"atariste", "520st"} {
		if mustLookup(t, c, s).ID != 42 {
			t.Fatalf("slug %q does not resolve to 42", s)
		}
	}
	if _, err := c.Lookup("ataristatariste520st"); err == nil {
		t.Fatal("joined-up full-name slug still resolves")
	}
}

func TestLookup_CacheHitMakesNoRequest(t *testing.T) {
	dir := t.TempDir()
	clock := &fakeClock{t0}
	src := &fakeSource{systems: []scraper.System{system(3, "NES,Famicom")}}
	mustLookup(t, newCatalog(src, dir, clock, nil), "nes")
	if src.calls != 1 {
		t.Fatalf("first run made %d requests, want 1", src.calls)
	}

	clock.now = t0.Add(11 * time.Hour)
	src2 := &fakeSource{}
	if p := mustLookup(t, newCatalog(src2, dir, clock, nil), "famicom"); p.ID != 3 {
		t.Fatalf("got %+v", p)
	}
	if src2.calls != 0 {
		t.Fatalf("fresh cache made %d requests, want 0", src2.calls)
	}
}

func TestLookup_ExpiredCacheIsRefetchedAndRewritten(t *testing.T) {
	dir := t.TempDir()
	clock := &fakeClock{t0}
	mustLookup(t, newCatalog(&fakeSource{systems: []scraper.System{system(3, "NES")}}, dir, clock, nil), "nes")

	clock.now = t0.Add(13 * time.Hour)
	src := &fakeSource{systems: []scraper.System{system(4, "Super Nintendo,SNES")}}
	if p := mustLookup(t, newCatalog(src, dir, clock, nil), "snes"); p.ID != 4 {
		t.Fatalf("got %+v", p)
	}
	if src.calls != 1 {
		t.Fatalf("expired cache made %d requests, want 1", src.calls)
	}

	// The rewritten file is fresh from the new fetch time.
	src3 := &fakeSource{}
	if p := mustLookup(t, newCatalog(src3, dir, clock, nil), "supernintendo"); p.ID != 4 || src3.calls != 0 {
		t.Fatalf("got %+v after %d requests", p, src3.calls)
	}
}

func TestLookup_OldFormatFileCountsAsExpired(t *testing.T) {
	dir := t.TempDir()
	old := `{"fetched_at": "` + t0.Format(time.RFC3339) + `",
	  "platforms": {"nes": {"ID": 99, "Name": "Old NES", "Extensions": [".nes"]}}}`
	if err := os.WriteFile(filepath.Join(dir, cacheFileName), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	clock := &fakeClock{t0.Add(time.Minute)}
	src := &fakeSource{systems: []scraper.System{system(3, "NES")}}
	if p := mustLookup(t, newCatalog(src, dir, clock, nil), "nes"); p.ID != 3 {
		t.Fatalf("got %+v, want the refetched platform", p)
	}
	if src.calls != 1 {
		t.Fatalf("made %d requests, want 1", src.calls)
	}

	// Refetched once: the file is now in the new format and fresh.
	src2 := &fakeSource{}
	mustLookup(t, newCatalog(src2, dir, clock, nil), "nes")
	if src2.calls != 0 {
		t.Fatalf("second run made %d requests, want 0", src2.calls)
	}
}

func TestLookup_CollisionGoesToLowestID(t *testing.T) {
	src := &fakeSource{systems: []scraper.System{
		system(80, "Shared,Eighty"),
		system(7, "Seven,Shared"),
		system(50, "Shared"),
	}}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, nil)
	if p := mustLookup(t, c, "shared"); p.ID != 7 {
		t.Fatalf("shared -> %d, want 7", p.ID)
	}
	if p := mustLookup(t, c, "eighty"); p.ID != 80 {
		t.Fatalf("eighty -> %d, want 80", p.ID)
	}
}

func TestLookup_AliasBeatsNameSlug(t *testing.T) {
	// Alias atari2600 belongs to ID 26; ID 1 also has a name producing it.
	src := &fakeSource{systems: []scraper.System{
		system(1, "Atari 2600"),
		system(26, "Atari2600 Variant,2600"),
	}}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, nil)
	if p := mustLookup(t, c, "atari2600"); p.ID != 26 {
		t.Fatalf("atari2600 -> %d, want 26", p.ID)
	}
}

func TestLookup_ConfiguredBeatsAPIAndWorksOffline(t *testing.T) {
	src := &fakeSource{err: errors.New("offline")}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, map[string]Platform{
		"NES": {ID: 99, Name: "My NES", Extensions: []string{".nes"}},
	})
	p := mustLookup(t, c, "nes")
	if p.ID != 99 || p.Name != "My NES" {
		t.Fatalf("got %+v", p)
	}
	if src.calls != 0 {
		t.Fatalf("configured lookup made %d requests", src.calls)
	}

	src2 := &fakeSource{systems: []scraper.System{system(3, "NES")}}
	c2 := newCatalog(src2, t.TempDir(), &fakeClock{t0}, map[string]Platform{"nes": {ID: 99, Name: "My NES"}})
	entries, err := c2.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Slug == "nes" && e.Platform.ID != 99 {
			t.Fatalf("listing gives nes to %d", e.Platform.ID)
		}
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	src := &fakeSource{systems: []scraper.System{system(3, "NES")}}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, map[string]Platform{"MyBox": {ID: 500, Name: "My Box"}})
	if p := mustLookup(t, c, "NeS"); p.ID != 3 {
		t.Fatalf("NeS -> %+v", p)
	}
	if p := mustLookup(t, c, "MYBOX"); p.ID != 500 {
		t.Fatalf("MYBOX -> %+v", p)
	}
}

func TestLookup_UnknownSlugNamesIt(t *testing.T) {
	c := newCatalog(&fakeSource{systems: []scraper.System{system(3, "NES")}}, t.TempDir(), &fakeClock{t0}, nil)
	_, err := c.Lookup("nosuchbox")
	if err == nil || !strings.Contains(err.Error(), "nosuchbox") {
		t.Fatalf("err = %v, want one naming the slug", err)
	}
}

func TestLookup_FetchFailureIsReported(t *testing.T) {
	c := newCatalog(&fakeSource{err: errors.New("boom")}, t.TempDir(), &fakeClock{t0}, nil)
	_, err := c.Lookup("nes")
	if err == nil || !strings.Contains(err.Error(), "nes") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestList_DisplaySlugsResolveToTheirRow(t *testing.T) {
	src := &fakeSource{systems: []scraper.System{
		system(80, "Shared,Eighty Long"),
		system(7, "Seven,Shared"),
		system(26, "Atari2600,A2600,2600"),
		system(9, "Same,Bb,Aa"),
	}}
	c := newCatalog(src, t.TempDir(), &fakeClock{t0}, map[string]Platform{"custom": {ID: 1000, Name: "Custom"}})
	entries, err := c.List()
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]Entry{}
	for _, e := range entries {
		got[e.Platform.ID] = e
		if e.Slug == "" {
			continue
		}
		if p := mustLookup(t, c, e.Slug); p.ID != e.Platform.ID {
			t.Errorf("listed slug %q resolves to %d, row is %d", e.Slug, p.ID, e.Platform.ID)
		}
	}
	want := map[int]string{80: "eightylong", 7: "seven", 26: "atari2600", 9: "aa", 1000: "custom"}
	for id, slug := range want {
		if got[id].Slug != slug {
			t.Errorf("ID %d listed as %q, want %q", id, got[id].Slug, slug)
		}
	}
	if !got[1000].FromConfig || got[7].FromConfig {
		t.Errorf("FromConfig wrong: %+v", got)
	}
}

func TestList_FreshCacheMakesNoRequest(t *testing.T) {
	dir := t.TempDir()
	clock := &fakeClock{t0}
	mustLookup(t, newCatalog(&fakeSource{systems: []scraper.System{system(3, "NES")}}, dir, clock, nil), "nes")
	src := &fakeSource{}
	if _, err := newCatalog(src, dir, clock, nil).List(); err != nil {
		t.Fatal(err)
	}
	if src.calls != 0 {
		t.Fatalf("List made %d requests on a fresh cache", src.calls)
	}
}

// legacySlugMap is the platforms.json format before the catalog: a map of
// every slug to the platform it picked.
type legacySlugMap struct {
	Platforms map[string]struct {
		ID         int
		Name       string
		Extensions []string
	} `json:"platforms"`
}

// slugExceptions are slugs from the old slug map that knowingly resolve to a
// different platform now (collisions now go to the lowest ID). Each was
// approved by the maintainer; keys are slugs, values the new platform ID.
var slugExceptions = map[string]int{}

func TestLegacySlugMap_SlugsStillResolve(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "slugmap-platforms.json"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy legacySlugMap
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}

	// Rebuild the systems list the old map was made from.
	byID := map[int]scraper.System{}
	for _, p := range legacy.Platforms {
		byID[p.ID] = system(p.ID, p.Name, p.Extensions...)
	}
	var systems []scraper.System
	for _, s := range byID {
		systems = append(systems, s)
	}
	c := newCatalog(&fakeSource{systems: systems}, t.TempDir(), &fakeClock{t0}, nil)

	checked := 0
	for slug, old := range legacy.Platforms {
		if strings.Contains(old.Name, ",") && slug == nameSlug(old.Name) {
			continue // joined-up full-name slug, dropped on purpose
		}
		want := old.ID
		if id, ok := slugExceptions[slug]; ok {
			want = id
		}
		p, err := c.Lookup(slug)
		if err != nil {
			t.Errorf("%q (was ID %d): %v", slug, old.ID, err)
			continue
		}
		if p.ID != want {
			t.Errorf("%q resolves to %d (%s), was %d (%s)", slug, p.ID, p.Name, old.ID, old.Name)
		}
		checked++
	}
	if checked < 500 {
		t.Fatalf("only %d slugs checked", checked)
	}
}
