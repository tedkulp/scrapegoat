package scraper

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGameCacheGetWithShortKeyIsNotCached(t *testing.T) {
	gc := NewGameCache(t.TempDir(), 12, time.Now)

	for _, md5 := range []string{"", "a"} {
		result := gc.Get(&ROMHashes{MD5: md5})
		if result.Found {
			t.Errorf("MD5 %q: expected no cache entry, got %+v", md5, result)
		}
	}
}

func TestGameCacheWritesWithShortKeyFailAndWriteNothing(t *testing.T) {
	dir := t.TempDir()
	gc := NewGameCache(dir, 12, time.Now)
	game := &Game{ID: "1"}

	writes := map[string]func(*ROMHashes) error{
		"Set":         func(h *ROMHashes) error { return gc.Set(h, game) },
		"SetNotFound": gc.SetNotFound,
		"SetNonGame":  func(h *ROMHashes) error { return gc.SetNonGame(h, game) },
	}

	for name, write := range writes {
		for _, md5 := range []string{"", "a"} {
			if err := write(&ROMHashes{MD5: md5}); err == nil {
				t.Errorf("%s with MD5 %q: expected error, got nil", name, md5)
			}
		}
	}

	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != dir {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking cache dir: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected cache dir to stay empty, found %v", files)
	}
}

func TestGameCacheRoundTrip(t *testing.T) {
	gc := NewGameCache(t.TempDir(), 12, time.Now)
	hashes := &ROMHashes{MD5: "abcdef0123456789"}

	if err := gc.Set(hashes, &Game{ID: "42"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	result := gc.Get(hashes)
	if !result.Found || result.Game == nil || result.Game.ID != "42" {
		t.Errorf("expected cached game 42, got %+v", result)
	}
}

// An entry as written before cutoffs existed: the format is unchanged, so old
// caches keep working, and GetAny reads it whatever its age.
const legacyEntry = `{
  "fetched_at": "2024-03-01T10:00:00Z",
  "game": {"id": "42", "noms": [{"region": "us", "text": "Zelda"}]},
  "not_found": false,
  "is_non_game": false
}`

func TestGameCacheReadsExistingEntriesWithGetAndGetAny(t *testing.T) {
	dir := t.TempDir()
	hashes := &ROMHashes{MD5: "abcdef0123456789"}
	path := filepath.Join(dir, "games", "ab", "abcdef0123456789.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(legacyEntry), 0o644); err != nil {
		t.Fatal(err)
	}

	fresh := NewGameCache(dir, 12, func() time.Time { return time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC) })
	if r := fresh.Get(hashes); !r.Found || r.Game == nil || r.Game.ID != "42" {
		t.Errorf("Get within expiry: got %+v", r)
	}

	later := NewGameCache(dir, 12, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if r := later.Get(hashes); r.Found {
		t.Errorf("Get after expiry: want not found, got %+v", r)
	}
	if r := later.GetAny(hashes); !r.Found || r.Game == nil || r.Game.ID != "42" {
		t.Errorf("GetAny after expiry: got %+v", r)
	}
}
