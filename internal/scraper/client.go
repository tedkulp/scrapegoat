package scraper

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

const (
	apiBaseURL = "https://www.screenscraper.fr/api2"
)

// Client handles API communication with ScreenScraper
type Client struct {
	httpClient   *http.Client
	devID        string
	devPassword  string
	userID       string
	userPassword string
	softwareName string
}

// NewClient creates a new ScreenScraper API client
func NewClient(devID, devPassword, userID, userPassword, softwareName string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		devID:        devID,
		devPassword:  devPassword,
		userID:       userID,
		userPassword: userPassword,
		softwareName: softwareName,
	}
}

// GetGameInfo fetches game information from ScreenScraper API with retry logic
func (c *Client) GetGameInfo(systemID int, romName string, hashes *ROMHashes) (*Game, error) {
	params := url.Values{}
	params.Set("devid", c.devID)
	params.Set("devpassword", c.devPassword)
	params.Set("ssid", c.userID)
	params.Set("sspassword", c.userPassword)
	params.Set("softname", c.softwareName)
	params.Set("output", "xml")
	params.Set("systemeid", fmt.Sprintf("%d", systemID))

	// Always include hashes if available - they are the primary lookup method
	if hashes != nil {
		params.Set("crc", hashes.CRC32)
		params.Set("md5", hashes.MD5)
		params.Set("sha1", hashes.SHA1)
		params.Set("romtaille", fmt.Sprintf("%d", hashes.Size))
	}

	// romnom is optional but can help with matching
	// For ZIP files, only include if we want filename-based matching as fallback
	baseName := filepath.Base(romName)
	params.Set("romnom", baseName)

	apiURL := fmt.Sprintf("%s/jeuInfos.php?%s", apiBaseURL, params.Encode())

	// Retry logic: 3 attempts with exponential backoff
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			lastErr = fmt.Errorf("API request failed: %w", err)
			continue
		}

		// Read body for potential reuse in error messages
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		// Handle rate limiting with longer backoff
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 429 {
			lastErr = fmt.Errorf("rate limited by API")
			time.Sleep(10 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var apiResp APIResponse
		if err := xml.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("failed to decode API response: %w", err)
			continue
		}

		if apiResp.Game == nil {
			return nil, fmt.Errorf("game not found in ScreenScraper database")
		}

		return apiResp.Game, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// DownloadMedia downloads a media file from the given URL
func (c *Client) DownloadMedia(mediaURL string) ([]byte, error) {
	resp, err := c.httpClient.Get(mediaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("media download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read media data: %w", err)
	}

	return data, nil
}

// SetTimeout sets the HTTP client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// GetDebugURL returns the actual API URL that will be called (for debugging)
// WARNING: Contains sensitive credentials - use carefully
func (c *Client) GetDebugURL(systemID int, romName string, hashes *ROMHashes) string {
	params := url.Values{}
	params.Set("devid", c.devID)
	params.Set("devpassword", c.devPassword)
	params.Set("ssid", c.userID)
	params.Set("sspassword", c.userPassword)
	params.Set("softname", c.softwareName)
	params.Set("output", "xml")
	params.Set("systemeid", fmt.Sprintf("%d", systemID))

	if hashes != nil {
		params.Set("crc", hashes.CRC32)
		params.Set("md5", hashes.MD5)
		params.Set("sha1", hashes.SHA1)
		params.Set("romtaille", fmt.Sprintf("%d", hashes.Size))
	}

	baseName := filepath.Base(romName)
	params.Set("romnom", baseName)

	return fmt.Sprintf("%s/jeuInfos.php?%s", apiBaseURL, params.Encode())
}
