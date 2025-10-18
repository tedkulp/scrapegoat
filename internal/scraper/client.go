package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	cache        *GameCache
	debug        bool
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
		cache:        nil, // Cache is optional
		debug:        false,
	}
}

// SetDebug enables debug logging
func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}

// SetCache sets the cache for the client
func (c *Client) SetCache(cacheDir string, expirationHours int) {
	c.cache = NewGameCache(cacheDir, expirationHours)
}

// GetGameInfo fetches game information from ScreenScraper API with retry logic
// If cache is enabled, checks cache first before making API call
// Returns: game, userInfo, cached (true if from cache), error
func (c *Client) GetGameInfo(systemID int, romName string, hashes *ROMHashes) (*Game, *UserInfo, bool, error) {
	// Check cache first if enabled
	if c.cache != nil && hashes != nil {
		result := c.cache.Get(hashes)
		if result.Found {
			if c.debug {
				if result.NotFound {
					fmt.Fprintf(os.Stderr, "[DEBUG] API: GetGameInfo - CACHED (not found)\n")
				} else if result.IsNonGame {
					fmt.Fprintf(os.Stderr, "[DEBUG] API: GetGameInfo - CACHED (non-game)\n")
				} else {
					fmt.Fprintf(os.Stderr, "[DEBUG] API: GetGameInfo - CACHED (game found)\n")
				}
			}
			// Return cached result even if it's a negative result (not found or non-game)
			// Caller will need to check if Game is nil
			return result.Game, nil, true, nil
		}
	}

	if c.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] API: GetGameInfo - CALLING API\n")
	}

	params := url.Values{}
	params.Set("devid", c.devID)
	params.Set("devpassword", c.devPassword)
	params.Set("ssid", c.userID)
	params.Set("sspassword", c.userPassword)
	params.Set("softname", c.softwareName)
	params.Set("output", "json")
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
	maxRetries := 1
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

		var apiResp GameInfoJSONResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("failed to decode API response: %w", err)
			continue
		}

		if apiResp.Response.Game == nil {
			// Cache the "not found" result to avoid repeated API calls
			if c.cache != nil && hashes != nil {
				if err := c.cache.SetNotFound(hashes); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to cache 'not found' result: %v\n", err)
				}
			}
			return nil, nil, false, fmt.Errorf("game not found in ScreenScraper database")
		}

		game := apiResp.Response.Game
		userInfo := &apiResp.Response.SSUser

		// Check if this is a non-game and cache appropriately
		if c.cache != nil && hashes != nil {
			if game.IsNonGame() {
				if err := c.cache.SetNonGame(hashes, game); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to cache non-game data: %v\n", err)
				}
			} else {
				if err := c.cache.Set(hashes, game); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to cache game data: %v\n", err)
				}
			}
		}

		return game, userInfo, false, nil
	}

	return nil, nil, false, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// DownloadMedia downloads a media file from the given URL
func (c *Client) DownloadMedia(mediaURL string) ([]byte, error) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] API: DownloadMedia - CALLING API (%s)\n", filepath.Base(mediaURL))
	}
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

// GetSystemsList fetches the list of all systems/platforms from ScreenScraper
func (c *Client) GetSystemsList() ([]System, error) {
	params := url.Values{}
	params.Set("devid", c.devID)
	params.Set("devpassword", c.devPassword)
	params.Set("ssid", c.userID)
	params.Set("sspassword", c.userPassword)
	params.Set("softname", c.softwareName)
	params.Set("output", "json")

	apiURL := fmt.Sprintf("%s/systemesListe.php?%s", apiBaseURL, params.Encode())

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

		var apiResp SystemsListJSONResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = fmt.Errorf("failed to decode API response: %w", err)
			continue
		}

		// Convert SystemJSON to System
		systems := make([]System, 0, len(apiResp.Response.Systems))
		for _, sysJSON := range apiResp.Response.Systems {
			sys := System{
				ID: sysJSON.ID,
			}

			// Build names list from JSON fields
			if sysJSON.Names.US != "" {
				sys.Names = append(sys.Names, Name{Region: "us", Text: sysJSON.Names.US})
			}
			if sysJSON.Names.EU != "" {
				sys.Names = append(sys.Names, Name{Region: "eu", Text: sysJSON.Names.EU})
			}
			// Use Common as world region
			if sysJSON.Names.Common != "" {
				sys.Names = append(sys.Names, Name{Region: "wor", Text: sysJSON.Names.Common})
			}

			// Parse extensions from comma-separated string
			if sysJSON.Extensions != "" {
				for _, ext := range strings.Split(sysJSON.Extensions, ",") {
					ext = strings.TrimSpace(ext)
					if ext != "" {
						if !strings.HasPrefix(ext, ".") {
							ext = "." + ext
						}
						sys.Extensions = append(sys.Extensions, ext)
					}
				}
			}

			systems = append(systems, sys)
		}

		return systems, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// GetUserInfo fetches user account information and API quota details
func (c *Client) GetUserInfo() (*UserInfo, error) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] API: GetUserInfo - CALLING API\n")
	}
	params := url.Values{}
	params.Set("devid", c.devID)
	params.Set("devpassword", c.devPassword)
	params.Set("ssid", c.userID)
	params.Set("sspassword", c.userPassword)
	params.Set("softname", c.softwareName)
	params.Set("output", "json")

	apiURL := fmt.Sprintf("%s/ssuserInfos.php?%s", apiBaseURL, params.Encode())

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp UserInfoJSONResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	return &apiResp.Response.SSUser, nil
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
	params.Set("output", "json")
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
