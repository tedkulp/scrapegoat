package metadata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

const existingGamelist = `<?xml version="1.0" encoding="UTF-8"?>
<gameList>
  <game id="7">
    <path>./Deferred.nes</path>
    <name>Deferred Game</name>
    <image>./images/Deferred.png</image>
    <playcount>12</playcount>
  </game>
  <game>
    <path>./Scraped.nes</path>
    <name>Old Name</name>
  </game>
  <game>
    <path>./Gone.nes</path>
    <name>Removed ROM</name>
  </game>
</gameList>
`

func TestCarryOver_KeepsDeferredEntriesUnchangedAndWritesNewOnes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gamelist.xml")
	if err := os.WriteFile(path, []byte(existingGamelist), 0o644); err != nil {
		t.Fatal(err)
	}

	existing, err := ReadExisting(path)
	if err != nil {
		t.Fatalf("ReadExisting: %v", err)
	}
	gen := NewGenerator()
	gen.AddGameFromScraper("/roms/Scraped.nes", &screenscraper.Game{Names: []screenscraper.Name{{Region: "us", Text: "New Name"}}})
	if !gen.CarryOver(existing, "/roms/Deferred.nes") {
		t.Error("CarryOver found no entry for Deferred.nes")
	}
	if gen.CarryOver(existing, "/roms/NoEntry.nes") {
		t.Error("CarryOver reported an entry for a ROM the file does not list")
	}
	if err := gen.WriteToFile(path); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	for _, want := range []string{
		`<game id="7">`,
		"<path>./Deferred.nes</path>",
		"<image>./images/Deferred.png</image>",
		"<playcount>12</playcount>",
		"<name>New Name</name>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("written gamelist lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"Old Name", "Removed ROM", "NoEntry"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("written gamelist has %q:\n%s", unwanted, out)
		}
	}

	// The written file reads back, and the carried entry is still there.
	again, err := ReadExisting(path)
	if err != nil {
		t.Fatalf("re-reading written gamelist: %v", err)
	}
	gen2 := NewGenerator()
	if !gen2.CarryOver(again, "/roms/Deferred.nes") || !gen2.CarryOver(again, "/roms/Scraped.nes") {
		t.Error("written gamelist does not read back by path")
	}
}

func TestReadExisting_MissingFileHasNoEntries(t *testing.T) {
	existing, err := ReadExisting(filepath.Join(t.TempDir(), "gamelist.xml"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if NewGenerator().CarryOver(existing, "/roms/A.nes") {
		t.Error("missing file gave an entry")
	}
}

func TestReadExisting_UnparseableFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gamelist.xml")
	os.WriteFile(path, []byte("<gameList><game>"), 0o644)
	if _, err := ReadExisting(path); err == nil {
		t.Error("want an error for a truncated gamelist")
	}
}
