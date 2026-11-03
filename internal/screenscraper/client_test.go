package screenscraper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (f *fakeClock) Now() time.Time { return f.now }

func (f *fakeClock) Sleep(d time.Duration) {
	f.sleeps = append(f.sleeps, d)
	f.now = f.now.Add(d)
}

var testCreds = Credentials{
	DevID:        "dev",
	DevPassword:  "devsecret",
	UserID:       "user",
	UserPassword: "usersecret",
	SoftwareName: "test",
}

type testHarness struct {
	client   *Client
	clock    *fakeClock
	calls    int
	requests []time.Time
}

func newHarness(t *testing.T, handler func(attempt int) *http.Response, opts ...Option) *testHarness {
	t.Helper()
	h := &testHarness{clock: newFakeClock()}
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		h.calls++
		h.requests = append(h.requests, h.clock.Now())
		return handler(h.calls), nil
	})}
	all := append([]Option{WithHTTPClient(httpClient), WithClock(h.clock)}, opts...)
	h.client = NewClient(testCreds, all...)
	return h
}

func newTestClient(handler func(attempt int) *http.Response) (*Client, *int, *[]time.Duration) {
	h := &testHarness{clock: newFakeClock()}
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		h.calls++
		return handler(h.calls), nil
	})}
	h.client = NewClient(testCreds, WithHTTPClient(httpClient), WithClock(h.clock))
	return h.client, &h.calls, &h.clock.sleeps
}

const matchBody = `{"response":{"jeu":{"id":"42","noms":[{"region":"us","text":"Zelda"}]}}}`

var testHashes = &ROMHashes{
	CRC32: "deadbeef",
	MD5:   "0123456789abcdef0123456789abcdef",
	SHA1:  "0123456789abcdef0123456789abcdef01234567",
	Size:  1024,
}

func TestLookupFreshMatch(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) })

	result, err := h.client.Lookup(3, "zelda.nes", testHashes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeMatch {
		t.Errorf("expected OutcomeMatch, got %v", result.Outcome)
	}
	if result.Game == nil || result.Game.ID != "42" {
		t.Errorf("expected game 42, got %+v", result.Game)
	}
	if result.Cached {
		t.Error("expected fresh lookup to report Cached=false")
	}
}

func TestLookupMakesThreeAttemptsBeforeGivingUp(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusInternalServerError, "boom")
	})

	_, err := c.Lookup(3, "game.nes", nil)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("unexpected error: %v", err)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second}
	if !slices.Equal(*sleeps, want) {
		t.Errorf("expected backoff %v, got %v", want, *sleeps)
	}
}

func TestLookupSucceedsOnThirdAttempt(t *testing.T) {
	c, calls, _ := newTestClient(func(attempt int) *http.Response {
		if attempt < 3 {
			return respond(http.StatusServiceUnavailable, "busy")
		}
		return respond(http.StatusOK, `{"response":{"jeu":{"id":"1"}}}`)
	})

	result, err := c.Lookup(3, "game.nes", nil)
	if err != nil {
		t.Fatalf("expected success on third attempt, got %v", err)
	}
	if result.Outcome != OutcomeMatch || result.Game == nil || result.Cached {
		t.Fatalf("expected uncached match, got %+v", result)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
}

func TestLookupSkipsRateLimitDelayOnFinalAttempt(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusTooManyRequests, "slow down")
	})

	_, err := c.Lookup(3, "game.nes", nil)
	if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	want := []time.Duration{10 * time.Second, 2 * time.Second, 10 * time.Second, 4 * time.Second}
	if !slices.Equal(*sleeps, want) {
		t.Errorf("expected sleeps %v, got %v", want, *sleeps)
	}
}

func TestLookupNotFoundIsNotRetried(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusOK, `{"response":{}}`)
	})

	result, err := c.Lookup(3, "game.nes", nil)
	if err != nil {
		t.Fatalf("not found is an outcome, not an error; got %v", err)
	}
	if result.Outcome != OutcomeNotFound || result.Game != nil {
		t.Errorf("expected not-found outcome with nil game, got %+v", result)
	}
	if *calls != 1 {
		t.Errorf("expected 1 attempt, got %d", *calls)
	}
	if len(*sleeps) != 0 {
		t.Errorf("expected no sleeps, got %v", *sleeps)
	}
}

func TestGetSystemsListMakesThreeAttemptsBeforeGivingUp(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusInternalServerError, "boom")
	})

	_, err := c.GetSystemsList()
	if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second}
	if !slices.Equal(*sleeps, want) {
		t.Errorf("expected backoff %v, got %v", want, *sleeps)
	}
}

func TestGetSystemsListSucceedsOnThirdAttempt(t *testing.T) {
	c, calls, _ := newTestClient(func(attempt int) *http.Response {
		if attempt < 3 {
			return respond(http.StatusServiceUnavailable, "busy")
		}
		return respond(http.StatusOK, `{"response":{"systemes":[{"id":3,"noms":{"nom_us":"NES"},"extensions":"nes,zip"}]}}`)
	})

	systems, err := c.GetSystemsList()
	if err != nil {
		t.Fatalf("expected success on third attempt, got %v", err)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	if len(systems) != 1 || systems[0].ID != 3 || !slices.Equal(systems[0].Extensions, []string{".nes", ".zip"}) {
		t.Errorf("unexpected systems: %+v", systems)
	}
}

func TestGetUserInfoSucceedsOnThirdAttempt(t *testing.T) {
	c, calls, sleeps := newTestClient(func(attempt int) *http.Response {
		if attempt < 3 {
			return respond(http.StatusInternalServerError, "boom")
		}
		return respond(http.StatusOK, `{"response":{"ssuser":{"id":"ted","requeststoday":"5"}}}`)
	})

	info, err := c.GetUserInfo()
	if err != nil {
		t.Fatalf("expected success on third attempt, got %v", err)
	}
	if info == nil || info.ID != "ted" || info.RequestsToday != "5" {
		t.Errorf("unexpected user info: %+v", info)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	want := []time.Duration{2 * time.Second, 4 * time.Second}
	if !slices.Equal(*sleeps, want) {
		t.Errorf("expected backoff %v, got %v", want, *sleeps)
	}
}

func TestGetUserInfoMakesThreeAttemptsBeforeGivingUp(t *testing.T) {
	c, calls, _ := newTestClient(func(int) *http.Response {
		return respond(http.StatusInternalServerError, "boom")
	})

	info, err := c.GetUserInfo()
	if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil user info, got %+v", info)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
}

func TestGetSystemsListSkipsRateLimitDelayOnFinalAttempt(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusTooManyRequests, "slow down")
	})

	_, err := c.GetSystemsList()
	if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Fatalf("unexpected error: %v", err)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
	want := []time.Duration{10 * time.Second, 2 * time.Second, 10 * time.Second, 4 * time.Second}
	if !slices.Equal(*sleeps, want) {
		t.Errorf("expected sleeps %v, got %v", want, *sleeps)
	}
}

func TestLookupOutcomesFreshThenCached(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		outcome  Outcome
		wantGame bool
	}{
		{"match", matchBody, OutcomeMatch, true},
		{"not found", `{"response":{}}`, OutcomeNotFound, false},
		{"non-game", `{"response":{"jeu":{"id":"7","noms":[{"region":"wor","text":"ZZZ(notgame):#NONGAME"}]}}}`, OutcomeNonGame, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, tc.body) },
				WithCache(t.TempDir(), 12))

			for i, wantCached := range []bool{false, true} {
				result, err := h.client.Lookup(3, "rom.nes", testHashes)
				if err != nil {
					t.Fatalf("lookup %d: expected nil error, got %v", i, err)
				}
				if result.Outcome != tc.outcome {
					t.Errorf("lookup %d: expected %v, got %v", i, tc.outcome, result.Outcome)
				}
				if result.Cached != wantCached {
					t.Errorf("lookup %d: expected Cached=%v, got %v", i, wantCached, result.Cached)
				}
				if (result.Game != nil) != tc.wantGame {
					t.Errorf("lookup %d: expected game present=%v, got %+v", i, tc.wantGame, result.Game)
				}
			}
			if h.calls != 1 {
				t.Errorf("expected 1 HTTP request, got %d", h.calls)
			}
		})
	}
}

func TestLookupCacheExpiresByClientClock(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) },
		WithCache(t.TempDir(), 12))

	if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}
	h.clock.now = h.clock.now.Add(13 * time.Hour)

	result, err := h.client.Lookup(3, "rom.nes", testHashes)
	if err != nil {
		t.Fatal(err)
	}
	if result.Cached {
		t.Error("expected entry older than the expiry to be refetched")
	}
	if h.calls != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", h.calls)
	}
}

func TestLookupFailuresReturnErrorAfterThreeAttempts(t *testing.T) {
	cases := map[string]roundTripFunc{
		"transport error": func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") },
		"HTTP 500": func(*http.Request) (*http.Response, error) {
			return respond(http.StatusInternalServerError, "boom"), nil
		},
		"bad JSON": func(*http.Request) (*http.Response, error) { return respond(http.StatusOK, "{not json"), nil },
	}

	for name, transport := range cases {
		t.Run(name, func(t *testing.T) {
			calls := 0
			hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return transport(r)
			})}
			c := NewClient(testCreds, WithHTTPClient(hc), WithClock(newFakeClock()))

			_, err := c.Lookup(3, "rom.nes", testHashes)
			if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
				t.Fatalf("expected failure after 3 attempts, got %v", err)
			}
			if calls != 3 {
				t.Errorf("expected 3 attempts, got %d", calls)
			}
		})
	}
}

func TestBackToBackLookupsAreThrottled(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) })

	for _, name := range []string{"a.nes", "b.nes"} {
		if _, err := h.client.Lookup(3, name, nil); err != nil {
			t.Fatal(err)
		}
	}

	if len(h.requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(h.requests))
	}
	if gap := h.requests[1].Sub(h.requests[0]); gap < time.Second {
		t.Errorf("expected at least 1s between requests, got %v", gap)
	}
}

func TestThrottleIntervalIsConfigurable(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) },
		WithThrottle(5*time.Second))

	if _, err := h.client.Lookup(3, "a.nes", nil); err != nil {
		t.Fatal(err)
	}
	h.clock.now = h.clock.now.Add(2 * time.Second)
	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}

	if gap := h.requests[1].Sub(h.requests[0]); gap < 5*time.Second {
		t.Errorf("expected at least 5s between requests, got %v", gap)
	}
	if !slices.Equal(h.clock.sleeps, []time.Duration{3 * time.Second}) {
		t.Errorf("expected a single 3s throttle wait, got %v", h.clock.sleeps)
	}
}

func TestThrottleAppliesAcrossAPICalls(t *testing.T) {
	h := newHarness(t, func(int) *http.Response {
		return respond(http.StatusOK, `{"response":{"systemes":[],"ssuser":{"id":"u"}}}`)
	})

	if _, err := h.client.GetSystemsList(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.client.Lookup(3, "a.nes", nil); err != nil {
		t.Fatal(err)
	}

	for i := 1; i < len(h.requests); i++ {
		if gap := h.requests[i].Sub(h.requests[i-1]); gap < time.Second {
			t.Errorf("request %d: expected at least 1s gap, got %v", i, gap)
		}
	}
}

func TestCachedLookupDoesNotWait(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) },
		WithCache(t.TempDir(), 12))

	if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}
	result, err := h.client.Lookup(3, "rom.nes", testHashes)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cached {
		t.Fatal("expected second lookup to be cached")
	}
	if len(h.clock.sleeps) != 0 {
		t.Errorf("expected no sleeps, got %v", h.clock.sleeps)
	}
}

const matchWithQuotaBody = `{"response":{"ssuser":{"id":"u","requeststoday":"150","maxrequestsperday":"20000"},"jeu":{"id":"42"}}}`

func TestQuotaIsUnknownBeforeAnyResponse(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchBody) })

	if q := h.client.Quota(); q.Known {
		t.Errorf("expected unknown quota, got %+v", q)
	}
}

func TestQuotaIsParsedFromLookupResponse(t *testing.T) {
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, matchWithQuotaBody) })

	if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}

	q := h.client.Quota()
	if !q.Known || q.RequestsToday != 150 || q.MaxRequestsPerDay != 20000 {
		t.Errorf("unexpected quota %+v", q)
	}
	if q.Remaining() != 19850 {
		t.Errorf("expected 19850 remaining, got %d", q.Remaining())
	}
}

func TestQuotaIsUnchangedByCacheHit(t *testing.T) {
	body := matchWithQuotaBody
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, body) },
		WithCache(t.TempDir(), 12))

	if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}
	before := h.client.Quota()
	body = `{"response":{"ssuser":{"requeststoday":"999","maxrequestsperday":"20000"},"jeu":{"id":"42"}}}`

	result, err := h.client.Lookup(3, "rom.nes", testHashes)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cached {
		t.Fatal("expected cached lookup")
	}
	if after := h.client.Quota(); after != before {
		t.Errorf("expected quota %+v to survive a cache hit, got %+v", before, after)
	}
}

func TestQuotaIsUnknownWhenNumbersDoNotParse(t *testing.T) {
	body := matchWithQuotaBody
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, body) })

	if _, err := h.client.Lookup(3, "a.nes", nil); err != nil {
		t.Fatal(err)
	}
	body = `{"response":{"ssuser":{"requeststoday":"lots","maxrequestsperday":"20000"},"jeu":{"id":"42"}}}`
	if _, err := h.client.Lookup(3, "b.nes", nil); err != nil {
		t.Fatal(err)
	}

	if q := h.client.Quota(); q.Known {
		t.Errorf("expected Known=false, got %+v", q)
	}
}

func TestQuotaIsUpdatedByUserInfoAndSystemsList(t *testing.T) {
	body := `{"response":{"ssuser":{"requeststoday":"10","maxrequestsperday":"100"}}}`
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, body) })

	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}
	if q := h.client.Quota(); !q.Known || q.RequestsToday != 10 || q.Remaining() != 90 {
		t.Errorf("expected quota from user info (10/100), got %+v", q)
	}

	body = `{"response":{"systemes":[],"ssuser":{"requeststoday":"50","maxrequestsperday":"100"}}}`
	if _, err := h.client.GetSystemsList(); err != nil {
		t.Fatal(err)
	}
	if q := h.client.Quota(); !q.Known || q.RequestsToday != 50 || q.Remaining() != 50 {
		t.Errorf("expected quota from systems list (50/100), got %+v", q)
	}
}

func TestSystemsListWithoutSSUserLeavesQuotaUnchanged(t *testing.T) {
	body := `{"response":{"ssuser":{"requeststoday":"10","maxrequestsperday":"100"}}}`
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, body) })

	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}
	body = `{"response":{"systemes":[]}}`
	if _, err := h.client.GetSystemsList(); err != nil {
		t.Fatal(err)
	}

	if q := h.client.Quota(); !q.Known || q.RequestsToday != 10 {
		t.Errorf("expected quota unchanged (10/100), got %+v", q)
	}
}

func TestRequestCountIncludesRetries(t *testing.T) {
	h := newHarness(t, func(attempt int) *http.Response {
		if attempt == 1 {
			return respond(http.StatusInternalServerError, "boom")
		}
		return respond(http.StatusOK, matchBody)
	}, WithCache(t.TempDir(), 12))

	for range 2 {
		if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
			t.Fatal(err)
		}
	}

	if got := h.client.RequestCount(); got != 2 {
		t.Errorf("expected 2 requests (one retry, no request for the cache hit), got %d", got)
	}
}

func TestDebugOutputLogsRequestURLWithoutPasswords(t *testing.T) {
	var out bytes.Buffer
	h := newHarness(t, func(attempt int) *http.Response {
		if attempt == 1 {
			return respond(http.StatusInternalServerError, "boom")
		}
		return respond(http.StatusOK, matchWithQuotaBody)
	}, WithDebug(true), WithDebugOutput(&out))

	if _, err := h.client.Lookup(3, "rom.nes", testHashes); err != nil {
		t.Fatal(err)
	}
	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.client.GetSystemsList(); err != nil {
		t.Fatal(err)
	}

	logged := out.String()
	for _, secret := range []string{testCreds.DevPassword, testCreds.UserPassword} {
		if strings.Contains(logged, secret) {
			t.Errorf("debug output leaks password %q:\n%s", secret, logged)
		}
	}
	for _, want := range []string{"jeuInfos.php?", "ssuserInfos.php?", "systemesListe.php?", "devpassword=***", "sspassword=***", "md5=" + testHashes.MD5} {
		if !strings.Contains(logged, want) {
			t.Errorf("expected debug output to contain %q:\n%s", want, logged)
		}
	}
}

func TestQuotaIsUnchangedByUserInfoResponseWithoutSSUser(t *testing.T) {
	body := `{"response":{"ssuser":{"requeststoday":"10","maxrequestsperday":"100"}}}`
	h := newHarness(t, func(int) *http.Response { return respond(http.StatusOK, body) })

	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}
	body = `{"response":{}}`
	if _, err := h.client.GetUserInfo(); err != nil {
		t.Fatal(err)
	}

	if q := h.client.Quota(); !q.Known || q.RequestsToday != 10 {
		t.Errorf("expected quota 10/100 to survive a response without ssuser, got %+v", q)
	}
}

func TestForbiddenFailsFastNamingTheCause(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusForbidden, "Erreur de login : Verifier vos identifiants developpeur !")
	})

	_, err := c.Lookup(3, "game.nes", testHashes)
	if err == nil {
		t.Fatal("expected an error for 403")
	}
	if *calls != 1 {
		t.Errorf("expected 1 request, got %d", *calls)
	}
	if len(*sleeps) != 0 {
		t.Errorf("expected no sleeps, got %v", *sleeps)
	}
	if !strings.Contains(err.Error(), "developer credentials (403)") {
		t.Errorf("expected error naming the cause and status, got %v", err)
	}
	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusForbidden {
		t.Errorf("expected *StatusError with status 403, got %#v", err)
	}
}

func TestPermanentStatusesAreNotRetried(t *testing.T) {
	for _, status := range []int{400, 403, 423, 426, 430, 431} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, calls, sleeps := newTestClient(func(int) *http.Response { return respond(status, "nope") })

			_, err := c.GetUserInfo()
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("(%d)", status)) {
				t.Fatalf("expected error naming status %d, got %v", status, err)
			}
			if *calls != 1 || len(*sleeps) != 0 {
				t.Errorf("expected 1 request and no sleeps, got %d requests, sleeps %v", *calls, *sleeps)
			}
		})
	}
}

func TestTransientStatusesAreRetried(t *testing.T) {
	for _, status := range []int{401, 408, 500, 502, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, calls, sleeps := newTestClient(func(int) *http.Response { return respond(status, "busy") })

			_, err := c.GetSystemsList()
			if err == nil || !strings.Contains(err.Error(), "failed after 3 attempts") {
				t.Fatalf("expected failure after 3 attempts, got %v", err)
			}
			if *calls != 3 {
				t.Errorf("expected 3 requests, got %d", *calls)
			}
			want := []time.Duration{2 * time.Second, 4 * time.Second}
			if !slices.Equal(*sleeps, want) {
				t.Errorf("expected backoff %v, got %v", want, *sleeps)
			}
		})
	}
}

func TestLookup404IsNotFoundOutcomeAndCached(t *testing.T) {
	h := newHarness(t, func(int) *http.Response {
		return respond(http.StatusNotFound, "Erreur : Rom/Iso/Dossier non trouvée !")
	}, WithCache(t.TempDir(), 24))

	result, err := h.client.Lookup(3, "unknown.nes", testHashes)
	if err != nil {
		t.Fatalf("404 from a lookup is an outcome, not an error; got %v", err)
	}
	if result.Outcome != OutcomeNotFound || result.Game != nil || result.Cached {
		t.Errorf("expected fresh not-found outcome, got %+v", result)
	}
	if h.calls != 1 || len(h.clock.sleeps) != 0 {
		t.Errorf("expected 1 request and no sleeps, got %d requests, sleeps %v", h.calls, h.clock.sleeps)
	}

	again, err := h.client.Lookup(3, "unknown.nes", testHashes)
	if err != nil || again.Outcome != OutcomeNotFound || !again.Cached {
		t.Errorf("expected cached not-found on second lookup, got %+v, %v", again, err)
	}
	if h.calls != 1 {
		t.Errorf("expected cached answer without a new request, got %d requests", h.calls)
	}
}

func Test404OutsideLookupIsAPermanentError(t *testing.T) {
	c, calls, _ := newTestClient(func(int) *http.Response { return respond(http.StatusNotFound, "") })

	_, err := c.GetUserInfo()
	if err == nil || !strings.Contains(err.Error(), "(404)") {
		t.Fatalf("expected error naming 404, got %v", err)
	}
	if *calls != 1 {
		t.Errorf("expected 1 request, got %d", *calls)
	}
}
