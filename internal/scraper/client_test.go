package scraper

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// roundTripFunc stubs the HTTP boundary so no real API calls are made.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// newTestClient returns a client whose HTTP calls go to handler and whose
// backoff sleeps are recorded instead of slept.
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
	if len(*sleeps) != len(want) || (*sleeps)[0] != want[0] || (*sleeps)[1] != want[1] {
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
