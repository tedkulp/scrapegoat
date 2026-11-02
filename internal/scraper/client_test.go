package scraper

import (
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

func newTestClient(handler func(attempt int) *http.Response) (*Client, *int, *[]time.Duration) {
	calls := 0
	var sleeps []time.Duration
	c := NewClient("dev", "devpw", "user", "userpw", "test")
	c.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return handler(calls), nil
	})}
	c.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	return c, &calls, &sleeps
}

func TestGetGameInfoMakesThreeAttemptsBeforeGivingUp(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusInternalServerError, "boom")
	})

	_, _, _, err := c.GetGameInfo(3, "game.nes", nil)
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

func TestGetGameInfoSucceedsOnThirdAttempt(t *testing.T) {
	c, calls, _ := newTestClient(func(attempt int) *http.Response {
		if attempt < 3 {
			return respond(http.StatusServiceUnavailable, "busy")
		}
		return respond(http.StatusOK, `{"response":{"jeu":{"id":"1"}}}`)
	})

	game, _, cached, err := c.GetGameInfo(3, "game.nes", nil)
	if err != nil {
		t.Fatalf("expected success on third attempt, got %v", err)
	}
	if game == nil || cached {
		t.Fatalf("expected uncached game, got game=%v cached=%v", game, cached)
	}
	if *calls != 3 {
		t.Errorf("expected 3 attempts, got %d", *calls)
	}
}

func TestGetGameInfoSkipsRateLimitDelayOnFinalAttempt(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusTooManyRequests, "slow down")
	})

	_, _, _, err := c.GetGameInfo(3, "game.nes", nil)
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

func TestGetGameInfoNotFoundIsNotRetried(t *testing.T) {
	c, calls, sleeps := newTestClient(func(int) *http.Response {
		return respond(http.StatusOK, `{"response":{}}`)
	})

	game, _, _, err := c.GetGameInfo(3, "game.nes", nil)
	if err == nil || !strings.Contains(err.Error(), "game not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
	if game != nil {
		t.Errorf("expected nil game, got %v", game)
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
