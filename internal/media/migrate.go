package media

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func systemCacheDir(cacheDir string, systemID int) string {
	return filepath.Join(cacheDir, "cache", strconv.Itoa(systemID))
}

// MigrateLegacyCache moves media cached under <cacheDir>/cache/<legacyName>,
// where media was cached by platform slug before it was keyed by system ID,
// into <cacheDir>/cache/<systemID>. A file the ID folder already has is kept
// there, and the legacy copy and its sidecar are dropped. The legacy folder is
// removed afterwards. It returns how many media files were moved.
func MigrateLegacyCache(cacheDir, legacyName string, systemID int) (int, error) {
	if legacyName == "" || legacyName == "." || legacyName == ".." ||
		filepath.Base(legacyName) != legacyName || legacyName == strconv.Itoa(systemID) {
		return 0, nil
	}
	legacy := filepath.Join(cacheDir, "cache", legacyName)
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return 0, nil
	}
	dest := systemCacheDir(cacheDir, systemID)

	moved := 0
	err := filepath.WalkDir(legacy, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".json") {
			return nil
		}
		rel, err := filepath.Rel(legacy, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Rename(path, target); err != nil {
			return err
		}
		moved++
		if _, err := os.Stat(sidecarPath(path)); err == nil {
			return os.Rename(sidecarPath(path), sidecarPath(target))
		}
		// A sidecar left at the target described a file that is gone.
		os.Remove(sidecarPath(target))
		return nil
	})
	if err != nil {
		return moved, err
	}
	return moved, os.RemoveAll(legacy)
}
