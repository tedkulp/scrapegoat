package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

func TestQuotaLinesShowsPerMinuteAndFailedRequestQuota(t *testing.T) {
	u := &scraper.UserInfo{
		RequestsToday:        "42",
		MaxRequestsPerDay:    "20000",
		MaxRequestsPerMinute: "3072",
		RequestsKOToday:      "7",
		MaxRequestsKOPerDay:  "2000",
		MaxThreads:           "1",
	}

	lines := quotaLines(u)

	for _, want := range []string{
		"Requests Today: 42",
		"Max Requests Per Day: 20000",
		"Max Requests Per Minute: 3072",
		"Failed Requests Today: 7 / 2000",
		"Max Threads: 1",
	} {
		if !slices.Contains(lines, want) {
			t.Errorf("missing line %q in %q", want, lines)
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "Per Hour") || strings.Contains(l, "Per Second") {
			t.Errorf("unexpected line %q", l)
		}
	}
}
