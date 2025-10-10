package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ROMFile represents a discovered ROM file
type ROMFile struct {
	Path     string
	Filename string
	Size     int64
}

// Scanner scans directories for ROM files
type Scanner struct {
	extensions map[string]bool // Supported file extensions
}

// NewScanner creates a new ROM scanner with supported extensions
func NewScanner(extensions []string) *Scanner {
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		// Normalize extension (lowercase, with leading dot)
		normalized := strings.ToLower(ext)
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + normalized
		}
		extMap[normalized] = true
	}

	return &Scanner{
		extensions: extMap,
	}
}

// Scan walks through directory and returns all matching ROM files
func (s *Scanner) Scan(directory string) ([]ROMFile, error) {
	var roms []ROMFile

	// Check if directory exists
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", directory)
	}

	// Walk directory tree
	err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has a supported extension
		ext := strings.ToLower(filepath.Ext(path))
		if s.extensions[ext] {
			roms = append(roms, ROMFile{
				Path:     path,
				Filename: filepath.Base(path),
				Size:     info.Size(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning directory: %w", err)
	}

	return roms, nil
}

// IsSupported checks if a file extension is supported
func (s *Scanner) IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return s.extensions[ext]
}
