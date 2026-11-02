package main

import (
	"testing"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

func TestLookupGame_CachedNotFoundIsReportedAsFailure(t *testing.T) {
	cacheDir := t.TempDir()
	hashes := &scraper.ROMHashes{
		CRC32: "deadbeef",
		MD5:   "0123456789abcdef0123456789abcdef",
		SHA1:  "0123456789abcdef0123456789abcdef01234567",
		Size:  1024,
	}

	// A previous run cached this ROM as "not found"
	if err := scraper.NewGameCache(cacheDir, 24).SetNotFound(hashes); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	client := scraper.NewClient("dev", "devpw", "user", "userpw", "test")
	client.SetCache(cacheDir, 24)

	game, _, cached, err := lookupGame(client, 3, "missing.nes", hashes)

	if err == nil {
		t.Fatal("expected an error for a cached not-found ROM, got nil")
	}
	if game != nil {
		t.Errorf("expected nil game, got %+v", game)
	}
	if !cached {
		t.Error("expected cached to be true")
	}
}
