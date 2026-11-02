package scraper

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// chdir switches into dir for the duration of the test so short relative
// paths resolve to files the test controls.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestHashROMFile_ShortPathDoesNotPanic(t *testing.T) {
	chdir(t, t.TempDir())
	if err := os.WriteFile("ab", []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	hashes, err := HashROMFile("ab")
	if err != nil {
		t.Fatalf("HashROMFile(\"ab\") error: %v", err)
	}
	// CRC32 of "abc".
	if hashes.CRC32 != "352441c2" {
		t.Errorf("CRC32 = %q, want %q", hashes.CRC32, "352441c2")
	}
	if hashes.Size != 3 {
		t.Errorf("Size = %d, want 3", hashes.Size)
	}
}

func TestHashROMFile_ShortMissingPathsReturnError(t *testing.T) {
	chdir(t, t.TempDir())
	for _, p := range []string{"", "a", "zip"} {
		if _, err := HashROMFile(p); err == nil {
			t.Errorf("HashROMFile(%q) error = nil, want error", p)
		}
	}
}

func TestHashROMFile_UppercaseZipHashesInnerROM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GAME.ZIP")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("game.nes")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	hashes, err := HashROMFile(path)
	if err != nil {
		t.Fatalf("HashROMFile error: %v", err)
	}
	if hashes.CRC32 != "352441c2" || hashes.Size != 3 {
		t.Errorf("got CRC32=%q Size=%d, want inner ROM CRC32=352441c2 Size=3", hashes.CRC32, hashes.Size)
	}
}
