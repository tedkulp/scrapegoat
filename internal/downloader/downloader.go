package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MediaFile represents a media file to download
type MediaFile struct {
	URL      string
	Filename string
	Type     string // e.g., "box-2D", "screenshot", "video"
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	MediaFile MediaFile
	LocalPath string
	Cached    bool // True if file was retrieved from cache
	Error     error
}

// Downloader handles concurrent media file downloads
type Downloader struct {
	httpClient  *http.Client
	cacheDir    string
	platform    string
	workerCount int
}

// NewDownloader creates a new media downloader
// cacheDir should be the base cache directory (e.g., ~/.scrapegoat/cache)
// platform is the platform name (e.g., "virtualboy", "nes")
func NewDownloader(cacheDir, platform string, workerCount int) (*Downloader, error) {
	// Build platform-specific cache directory
	platformCacheDir := filepath.Join(cacheDir, "cache", platform)

	// Ensure cache directory exists
	if err := os.MkdirAll(platformCacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Downloader{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		cacheDir:    platformCacheDir,
		platform:    platform,
		workerCount: workerCount,
	}, nil
}

// Download downloads media files concurrently
func (d *Downloader) Download(mediaFiles []MediaFile) []DownloadResult {
	if len(mediaFiles) == 0 {
		return nil
	}

	// Create channels
	jobs := make(chan MediaFile, len(mediaFiles))
	results := make(chan DownloadResult, len(mediaFiles))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < d.workerCount; i++ {
		wg.Add(1)
		go d.worker(&wg, jobs, results)
	}

	// Send jobs
	for _, mediaFile := range mediaFiles {
		jobs <- mediaFile
	}
	close(jobs)

	// Wait for workers to finish
	wg.Wait()
	close(results)

	// Collect results
	var downloadResults []DownloadResult
	for result := range results {
		downloadResults = append(downloadResults, result)
	}

	return downloadResults
}

// worker processes download jobs
func (d *Downloader) worker(wg *sync.WaitGroup, jobs <-chan MediaFile, results chan<- DownloadResult) {
	defer wg.Done()

	for mediaFile := range jobs {
		localPath, cached, err := d.downloadFile(mediaFile)
		results <- DownloadResult{
			MediaFile: mediaFile,
			LocalPath: localPath,
			Cached:    cached,
			Error:     err,
		}
	}
}

// downloadFile downloads a single media file or returns cached version
// Returns: localPath, cached (true if from cache), error
func (d *Downloader) downloadFile(mediaFile MediaFile) (string, bool, error) {
	// Build cache path: cache/{platform}/{media-type}/{filename}
	mediaTypeDir := filepath.Join(d.cacheDir, mediaFile.Type)
	if err := os.MkdirAll(mediaTypeDir, 0o755); err != nil {
		return "", false, fmt.Errorf("failed to create media type directory: %w", err)
	}

	localPath := filepath.Join(mediaTypeDir, mediaFile.Filename)

	// Check if file already exists in cache
	if _, err := os.Stat(localPath); err == nil {
		// File exists in cache, return it
		return localPath, true, nil
	}

	// File not in cache, download it
	resp, err := d.httpClient.Get(mediaFile.URL)
	if err != nil {
		return "", false, fmt.Errorf("failed to download %s: %w", mediaFile.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("download failed with status %d for %s", resp.StatusCode, mediaFile.URL)
	}

	// Create local file
	outFile, err := os.Create(localPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to create file %s: %w", localPath, err)
	}
	defer outFile.Close()

	// Copy data
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to write file %s: %w", localPath, err)
	}

	return localPath, false, nil
}

// DownloadSingle downloads a single file synchronously
func (d *Downloader) DownloadSingle(mediaFile MediaFile) (string, error) {
	path, _, err := d.downloadFile(mediaFile)
	return path, err
}

// CopyToFinal copies files from temp directory to final destination
func (d *Downloader) CopyToFinal(tempPath, finalPath string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(finalPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	src, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy data
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Cleanup is deprecated - cache directory is now persistent
// Kept for backward compatibility but does nothing
func (d *Downloader) Cleanup() error {
	return nil
}
