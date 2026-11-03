package cutoff

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestIsCutoffStatus(t *testing.T) {
	for _, code := range []int{403, 423, 426, 430, 431} {
		if !IsCutoffStatus(code) {
			t.Errorf("IsCutoffStatus(%d) = false, want true", code)
		}
	}
	for _, code := range []int{200, 400, 401, 404, 408, 429, 500, 503} {
		if IsCutoffStatus(code) {
			t.Errorf("IsCutoffStatus(%d) = true, want false", code)
		}
	}
}

func TestCutoff_RecordsOnlyTheFirstCause(t *testing.T) {
	c := New()
	if _, ok := c.Cause(); ok {
		t.Fatal("new cutoff reports a cause")
	}
	if !c.Trip(430, "quota") {
		t.Error("first Trip returned false")
	}
	if c.Trip(403, "creds") {
		t.Error("second Trip returned true")
	}
	cause, ok := c.Cause()
	if !ok || cause.StatusCode != 430 || cause.Body != "quota" {
		t.Errorf("Cause() = %+v, %v; want 430 quota", cause, ok)
	}
	if !c.Tripped() {
		t.Error("Tripped() = false after Trip")
	}
}

func TestCutoff_FirstOfConcurrentTripsWins(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	wins := make(chan int, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if c.Trip(430, fmt.Sprint(i)) {
				wins <- i
			}
		}(i)
	}
	wg.Wait()
	close(wins)
	if n := len(wins); n != 1 {
		t.Fatalf("%d Trip calls won, want 1", n)
	}
	won := <-wins
	if cause, _ := c.Cause(); cause.Body != fmt.Sprint(won) {
		t.Errorf("Cause body %q, want the winner's %q", cause.Body, fmt.Sprint(won))
	}
}

func TestCause_StringNamesTheCauseAndStatus(t *testing.T) {
	got := Cause{StatusCode: 430, Body: "ignored here"}.String()
	want := "ScreenScraper daily quota exceeded (430)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestError_MatchesWithErrorsAsAndUnwraps(t *testing.T) {
	inner := errors.New("status 430")
	var err error = fmt.Errorf("lookup: %w", &Error{Cause: Cause{StatusCode: 430}, Err: inner})
	var ce *Error
	if !errors.As(err, &ce) || ce.Cause.StatusCode != 430 {
		t.Fatalf("errors.As failed: %v", err)
	}
	if !errors.Is(err, inner) {
		t.Error("cutoff error does not unwrap to the response error")
	}
}
