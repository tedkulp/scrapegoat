package scraper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGameCacheGetWithEmptyKeyIsMiss(t *testing.T) {
	gc := NewGameCache(t.TempDir(), 12)

	for _, md5 := range []string{"", "a"} {
		result := gc.Get(&ROMHashes{MD5: md5})
		if result.Found {
			t.Errorf("MD5 %q: expected miss, got %+v", md5, result)
		}
	}
}

func TestGameCacheWritesWithEmptyKeyFailAndWriteNothing(t *testing.T) {
	dir := t.TempDir()
	gc := NewGameCache(dir, 12)
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
	gc := NewGameCache(t.TempDir(), 12)
	hashes := &ROMHashes{MD5: "abcdef0123456789"}

	if err := gc.Set(hashes, &Game{ID: "42"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	result := gc.Get(hashes)
	if !result.Found || result.Game == nil || result.Game.ID != "42" {
		t.Errorf("expected cached game 42, got %+v", result)
	}
}
