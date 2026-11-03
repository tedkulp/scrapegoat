package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyCache_RenamesSlugFolderToSystemID(t *testing.T) {
	cacheDir := t.TempDir()
	legacy := filepath.Join(cacheDir, "cache", "nes")
	writeFile(t, filepath.Join(legacy, "screenshots", "Game.png"), "shot")
	writeFile(t, filepath.Join(legacy, "screenshots", "Game.png.json"), "sidecar")
	writeFile(t, filepath.Join(legacy, "wheel", "Game.png"), "wheel")

	moved, err := MigrateLegacyCache(cacheDir, "nes", 3)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want 2", moved)
	}
	dest := filepath.Join(cacheDir, "cache", "3")
	if got := readFile(t, filepath.Join(dest, "screenshots", "Game.png")); got != "shot" {
		t.Errorf("screenshot = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "screenshots", "Game.png.json")); got != "sidecar" {
		t.Errorf("sidecar = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "wheel", "Game.png")); got != "wheel" {
		t.Errorf("wheel = %q", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy folder still there: %v", err)
	}
}

func TestMigrateLegacyCache_KeepsFilesTheIDFolderAlreadyHas(t *testing.T) {
	cacheDir := t.TempDir()
	legacy := filepath.Join(cacheDir, "cache", "famicom")
	dest := filepath.Join(cacheDir, "cache", "3")
	writeFile(t, filepath.Join(legacy, "screenshots", "Game.png"), "old")
	writeFile(t, filepath.Join(legacy, "screenshots", "Game.png.json"), "old-sidecar")
	writeFile(t, filepath.Join(legacy, "screenshots", "Other.png"), "other")
	writeFile(t, filepath.Join(legacy, "screenshots", "Other.png.json"), "other-sidecar")
	writeFile(t, filepath.Join(dest, "screenshots", "Game.png"), "new")

	moved, err := MigrateLegacyCache(cacheDir, "famicom", 3)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Errorf("moved = %d, want 1", moved)
	}
	if got := readFile(t, filepath.Join(dest, "screenshots", "Game.png")); got != "new" {
		t.Errorf("existing file overwritten: %q", got)
	}
	// The legacy sidecar describes the legacy file, not the one kept.
	if _, err := os.Stat(filepath.Join(dest, "screenshots", "Game.png.json")); !os.IsNotExist(err) {
		t.Errorf("legacy sidecar moved next to a different file: %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "screenshots", "Other.png.json")); got != "other-sidecar" {
		t.Errorf("sidecar = %q", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy folder still there: %v", err)
	}
}

func TestMigrateLegacyCache_SkipsTempFiles(t *testing.T) {
	cacheDir := t.TempDir()
	writeFile(t, filepath.Join(cacheDir, "cache", "nes", "screenshots", ".Game.png.123.tmp"), "partial")

	moved, err := MigrateLegacyCache(cacheDir, "nes", 3)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Errorf("moved = %d, want 0", moved)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "cache", "3", "screenshots", ".Game.png.123.tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file moved: %v", err)
	}
}

func TestMigrateLegacyCache_NothingToDo(t *testing.T) {
	cacheDir := t.TempDir()
	dest := filepath.Join(cacheDir, "cache", "3", "screenshots", "Game.png")
	writeFile(t, dest, "keep")

	for _, name := range []string{"nes", "3", "", ".", "..", "a/b"} {
		moved, err := MigrateLegacyCache(cacheDir, name, 3)
		if err != nil || moved != 0 {
			t.Errorf("%q: moved %d, err %v; want 0, nil", name, moved, err)
		}
	}
	if got := readFile(t, dest); got != "keep" {
		t.Errorf("ID folder changed: %q", got)
	}
}

func TestNew_CachesUnderSystemID(t *testing.T) {
	p := New("/c", 42, "/m", 1)
	if want := filepath.Join("/c", "cache", "42"); p.cacheDir != want {
		t.Errorf("cacheDir = %q, want %q", p.cacheDir, want)
	}
}
