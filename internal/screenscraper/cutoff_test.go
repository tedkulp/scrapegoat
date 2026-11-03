package screenscraper

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
)

var otherHashes = &ROMHashes{
	CRC32: "cafebabe",
	MD5:   "fedcba9876543210fedcba9876543210",
	SHA1:  "fedcba9876543210fedcba9876543210fedcba98",
	Size:  2048,
}

func TestCutoffStatusStopsAllLaterRequests(t *testing.T) {
	for _, status := range []int{403, 423, 426, 430, 431} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			cut := cutoff.New()
			h := newHarness(t, func(int) *http.Response { return respond(status, "refused") },
				WithCache(t.TempDir(), 12), WithCutoff(cut))

			_, err := h.client.Lookup(3, "first.nes", testHashes)
			var ce *cutoff.Error
			if !errors.As(err, &ce) || ce.Cause.StatusCode != status {
				t.Fatalf("first lookup: want *cutoff.Error with status %d, got %v", status, err)
			}
			var se *StatusError
			if !errors.As(err, &se) || se.StatusCode != status {
				t.Errorf("tripping lookup should wrap the *StatusError, got %#v", err)
			}
			if cause, ok := cut.Cause(); !ok || cause.StatusCode != status {
				t.Errorf("shared cutoff = %+v, %v; want status %d", cause, ok, status)
			}
			before := h.client.RequestCount()

			_, err = h.client.Lookup(3, "second.nes", otherHashes)
			if !errors.As(err, &ce) || ce.Cause.StatusCode != status {
				t.Fatalf("lookup after cutoff: want *cutoff.Error, got %v", err)
			}
			if got := h.client.RequestCount(); got != before {
				t.Errorf("request count went from %d to %d after the cutoff", before, got)
			}
			if _, err := h.client.GetUserInfo(); !errors.As(err, &ce) {
				t.Errorf("GetUserInfo after cutoff: want *cutoff.Error, got %v", err)
			}
			if h.calls != 1 {
				t.Errorf("want 1 HTTP request in all, got %d", h.calls)
			}
		})
	}
}

func TestCutoffTrippedElsewhereStopsLookupBeforeAnyRequest(t *testing.T) {
	cut := cutoff.New()
	cut.Trip(430, "quota")
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) }, WithCutoff(cut))

	_, err := h.client.Lookup(3, "rom.nes", testHashes)
	var ce *cutoff.Error
	if !errors.As(err, &ce) || ce.Err != nil {
		t.Fatalf("want *cutoff.Error with no response error, got %#v", err)
	}
	if h.calls != 0 {
		t.Errorf("want no requests, got %d", h.calls)
	}
}

func TestExpiredCacheEntryAnswersAfterCutoffButNotBefore(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		outcome Outcome
	}{
		{"match", matchBody, OutcomeMatch},
		{"not found", `{"response":{}}`, OutcomeNotFound},
		{"non-game", `{"response":{"jeu":{"id":"7","noms":[{"region":"wor","text":"ZZZ(notgame):#NONGAME"}]}}}`, OutcomeNonGame},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			// Fill the cache, then let the entry expire.
			seed := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, tc.body) },
				WithCache(cacheDir, 12))
			if _, err := seed.client.Lookup(3, "rom.nes", testHashes); err != nil {
				t.Fatal(err)
			}
			expired := seed.clock.now.Add(13 * time.Hour)

			// Before a cutoff the expired entry causes a request.
			before := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, tc.body) },
				WithCache(cacheDir, 12))
			before.clock.now = expired
			if res, err := before.client.Lookup(3, "rom.nes", testHashes); err != nil || res.Cached {
				t.Fatalf("before cutoff: want a fresh answer, got %+v, %v", res, err)
			}
			if before.calls != 1 {
				t.Fatalf("before cutoff: want 1 request, got %d", before.calls)
			}

			// Re-expire it (the lookup above refreshed it) and cut off.
			after := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, tc.body) },
				WithCache(cacheDir, 12), WithCutoff(tripped()))
			after.clock.now = expired.Add(13 * time.Hour)
			res, err := after.client.Lookup(3, "rom.nes", testHashes)
			if err != nil {
				t.Fatalf("after cutoff: want the cached outcome, got %v", err)
			}
			if res.Outcome != tc.outcome || !res.Cached {
				t.Errorf("after cutoff: got %v cached=%v, want %v cached", res.Outcome, res.Cached, tc.outcome)
			}
			if after.calls != 0 {
				t.Errorf("after cutoff: want no requests, got %d", after.calls)
			}
		})
	}
}

func TestExpiredCacheEntryAnswersTheLookupThatTripsTheCutoff(t *testing.T) {
	cacheDir := t.TempDir()
	seed := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) }, WithCache(cacheDir, 12))
	if _, err := seed.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}
	cut := cutoff.New()
	h := newHarness(t, func(int) *http.Response { return respond(430, "quota") }, WithCache(cacheDir, 12), WithCutoff(cut))
	h.clock.now = seed.clock.now.Add(13 * time.Hour)

	res, err := h.client.Lookup(3, "rom.nes", testHashes)
	if err != nil || res.Outcome != OutcomeMatch || !res.Cached {
		t.Fatalf("want cached match, got %+v, %v", res, err)
	}
	if !cut.Tripped() {
		t.Error("cutoff not recorded")
	}
}

func TestNonCutoffStatusesDoNotTripTheCutoff(t *testing.T) {
	for _, status := range []int{400, 404, 401, 408, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			cut := cutoff.New()
			h := newHarness(t, func(int) *http.Response { return respond(status, "nope") }, WithCutoff(cut))

			_, err := h.client.Lookup(3, "rom.nes", testHashes)
			var ce *cutoff.Error
			if errors.As(err, &ce) {
				t.Errorf("status %d gave a cutoff error: %v", status, err)
			}
			if cut.Tripped() {
				t.Errorf("status %d tripped the cutoff", status)
			}
		})
	}
}

func TestClientWithoutSharedCutoffStillStopsAfterOne(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(430, "quota") })
	h.client.Lookup(3, "a.nes", testHashes)
	_, err := h.client.Lookup(3, "b.nes", otherHashes)
	var ce *cutoff.Error
	if !errors.As(err, &ce) || h.calls != 1 {
		t.Errorf("want cutoff error and 1 request, got %v and %d", err, h.calls)
	}
}

func tripped() *cutoff.Cutoff {
	c := cutoff.New()
	c.Trip(430, "quota")
	return c
}
