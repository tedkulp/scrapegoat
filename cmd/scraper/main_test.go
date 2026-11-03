package main

import (
	"testing"

	"github.com/tedkulp/scrapegoat/internal/media"
)

func TestMediaSummary_ListsNonZeroCountsIncludingFailed(t *testing.T) {
	got := mediaSummary(media.Result{Existing: 2, FromCache: 1, Failed: 3})
	want := "2 existing, 1 from cache, 3 failed"
	if got != want {
		t.Errorf("mediaSummary = %q, want %q", got, want)
	}
}

func TestMediaSummary_ListsDeferredFiles(t *testing.T) {
	got := mediaSummary(media.Result{FromCache: 1, Deferred: 4})
	want := "1 from cache, 4 deferred (cutoff)"
	if got != want {
		t.Errorf("mediaSummary = %q, want %q", got, want)
	}
}

func TestMediaSummary_EmptyWhenNothingPlaced(t *testing.T) {
	if got := mediaSummary(media.Result{}); got != "" {
		t.Errorf("mediaSummary = %q, want empty", got)
	}
}

func TestCacheDurationMessage_UsesConfiguredHours(t *testing.T) {
	got := cacheDurationMessage(48)
	want := "Game data is cached for 48 hours"
	if got != want {
		t.Errorf("cacheDurationMessage(48) = %q, want %q", got, want)
	}
}

func TestCacheDurationMessage_SingleHour(t *testing.T) {
	got := cacheDurationMessage(1)
	want := "Game data is cached for 1 hour"
	if got != want {
		t.Errorf("cacheDurationMessage(1) = %q, want %q", got, want)
	}
}
