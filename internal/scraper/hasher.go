package scraper

import (
	"archive/zip"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ROMHashes contains all hash values for a ROM file
type ROMHashes struct {
	CRC32 string
	MD5   string
	SHA1  string
	Size  int64
}

// HashROMFile calculates CRC32, MD5, and SHA1 hashes for a ROM file
// If the file is a ZIP archive, it hashes the ROM file inside the archive
func HashROMFile(filepath string) (*ROMHashes, error) {
	// Check if this is a ZIP file
	if strings.ToLower(filepath[len(filepath)-4:]) == ".zip" {
		return hashZippedROM(filepath)
	}

	// Regular file hashing
	return hashFile(filepath)
}

// hashFile computes hashes for a regular file
func hashFile(filepath string) (*ROMHashes, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Create hash writers
	crc32Hash := crc32.NewIEEE()
	md5Hash := md5.New()
	sha1Hash := sha1.New()

	// Use MultiWriter to compute all hashes in one pass
	multiWriter := io.MultiWriter(crc32Hash, md5Hash, sha1Hash)

	// Read file and compute hashes
	if _, err := io.Copy(multiWriter, file); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &ROMHashes{
		CRC32: fmt.Sprintf("%08x", crc32Hash.Sum32()),
		MD5:   hex.EncodeToString(md5Hash.Sum(nil)),
		SHA1:  hex.EncodeToString(sha1Hash.Sum(nil)),
		Size:  fileInfo.Size(),
	}, nil
}

// hashZippedROM extracts and hashes the ROM file from a ZIP archive
func hashZippedROM(zipPath string) (*ROMHashes, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Common ROM extensions to look for inside ZIP
	romExtensions := map[string]bool{
		".nes": true, ".sfc": true, ".smc": true, ".md": true, ".bin": true,
		".gen": true, ".n64": true, ".z64": true, ".v64": true, ".gba": true,
		".gb": true, ".gbc": true, ".a26": true, ".a78": true, ".lnx": true,
		".pce": true, ".sms": true, ".gg": true, ".sg": true, ".cue": true,
		".iso": true, ".img": true, ".pbp": true, ".32x": true, ".nds": true,
	}

	// Find the first ROM file in the archive
	var romFile *zip.File
	for _, file := range reader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name))
		if romExtensions[ext] {
			romFile = file
			break
		}
	}

	if romFile == nil {
		// If no known ROM extension found, use the first non-directory file
		for _, file := range reader.File {
			if !file.FileInfo().IsDir() {
				romFile = file
				break
			}
		}
	}

	if romFile == nil {
		return nil, fmt.Errorf("no ROM file found inside ZIP archive")
	}

	// Open the ROM file from the archive
	rc, err := romFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open ROM file from archive: %w", err)
	}
	defer rc.Close()

	// Create hash writers
	crc32Hash := crc32.NewIEEE()
	md5Hash := md5.New()
	sha1Hash := sha1.New()

	// Use MultiWriter to compute all hashes in one pass
	multiWriter := io.MultiWriter(crc32Hash, md5Hash, sha1Hash)

	// Read ROM file and compute hashes
	size, err := io.Copy(multiWriter, rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM file from archive: %w", err)
	}

	return &ROMHashes{
		CRC32: fmt.Sprintf("%08x", crc32Hash.Sum32()),
		MD5:   hex.EncodeToString(md5Hash.Sum(nil)),
		SHA1:  hex.EncodeToString(sha1Hash.Sum(nil)),
		Size:  size,
	}, nil
}
