package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/downloader"
)

func TestFilterExistingMediaFiles_AllMissing(t *testing.T) {
	dir := t.TempDir()

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 0 {
		t.Errorf("expected 0 existing, got %d", len(existing))
	}
	if len(toDownload) != 2 {
		t.Errorf("expected 2 to download, got %d", len(toDownload))
	}
}

func TestFilterExistingMediaFiles_SomeExist(t *testing.T) {
	dir := t.TempDir()

	// Create the wheel file in its final location
	wheelDir := filepath.Join(dir, "wheel")
	if err := os.MkdirAll(wheelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wheelFile := filepath.Join(wheelDir, "game.png")
	if err := os.WriteFile(wheelFile, []byte("fake image"), 0o644); err != nil {
		t.Fatal(err)
	}

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 1 {
		t.Errorf("expected 1 existing, got %d", len(existing))
	}
	if existing["wheel"] != wheelFile {
		t.Errorf("expected existing[wheel] = %q, got %q", wheelFile, existing["wheel"])
	}
	if len(toDownload) != 1 {
		t.Errorf("expected 1 to download, got %d", len(toDownload))
	}
	if toDownload[0].Type != "screenshots" {
		t.Errorf("expected toDownload[0].Type = screenshots, got %s", toDownload[0].Type)
	}
}

func TestFilterExistingMediaFiles_AllExist(t *testing.T) {
	dir := t.TempDir()

	for _, mediaType := range []string{"wheel", "screenshots"} {
		typeDir := filepath.Join(dir, mediaType)
		if err := os.MkdirAll(typeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(typeDir, "game.png"), []byte("fake"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mediaFiles := []downloader.MediaFile{
		{URL: "http://example.com/a.png", Filename: "game.png", Type: "wheel"},
		{URL: "http://example.com/b.png", Filename: "game.png", Type: "screenshots"},
	}

	existing, toDownload := filterExistingMediaFiles(mediaFiles, dir)

	if len(existing) != 2 {
		t.Errorf("expected 2 existing, got %d", len(existing))
	}
	if len(toDownload) != 0 {
		t.Errorf("expected 0 to download, got %d", len(toDownload))
	}
}

func TestFilterExistingMediaFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	existing, toDownload := filterExistingMediaFiles(nil, dir)
	if len(existing) != 0 {
		t.Errorf("expected 0 existing, got %d", len(existing))
	}
	if len(toDownload) != 0 {
		t.Errorf("expected 0 to download, got %d", len(toDownload))
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
