package screenscraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tedkulp/scrapegoat/internal/cutoff"
)

const (
	apiBaseURL      = "https://www.screenscraper.fr/api2"
	maxAttempts     = 3
	rateLimitDelay  = 10 * time.Second
	defaultThrottle = time.Second
)

// Credentials identify both the developer and the user to ScreenScraper.
type Credentials struct {
	DevID        string
	DevPassword  string
	UserID       string
	UserPassword string
	SoftwareName string
}

func (cr Credentials) params() url.Values {
	params := url.Values{}
	params.Set("devid", cr.DevID)
	params.Set("devpassword", cr.DevPassword)
	params.Set("ssid", cr.UserID)
	params.Set("sspassword", cr.UserPassword)
	params.Set("softname", cr.SoftwareName)
	params.Set("output", "json")
	return params
}

// Clock is the time source the client uses for retry backoff, throttling and
// cache expiry.
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type systemClock struct{}

func (systemClock) Now() time.Time        { return time.Now() }
func (systemClock) Sleep(d time.Duration) { time.Sleep(d) }

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets the HTTP client used to reach ScreenScraper.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithClock sets the clock used for retry backoff, throttling and cache expiry.
func WithClock(clock Clock) Option {
	return func(c *Client) { c.clock = clock }
}

// WithCache enables the on-disk cache of lookup answers.
func WithCache(dir string, expirationHours int) Option {
	return func(c *Client) {
		c.cacheDir = dir
		c.cacheHours = expirationHours
		c.cacheEnabled = true
	}
}

// WithThrottle sets the minimum gap between consecutive ScreenScraper requests.
func WithThrottle(gap time.Duration) Option {
	return func(c *Client) { c.throttle = gap }
}

// WithCutoff shares a run's cutoff with the client. The client trips it on a
// cutoff status and sends no request once it is tripped, by whoever. Without
// this option the client keeps a cutoff of its own.
func WithCutoff(cut *cutoff.Cutoff) Option {
	return func(c *Client) { c.cutoff = cut }
}

// WithDebug enables debug logging of lookups and request URLs.
func WithDebug(debug bool) Option {
	return func(c *Client) { c.debug = debug }
}

// WithDebugOutput sets where debug logging is written (stderr by default).
func WithDebugOutput(w io.Writer) Option {
	return func(c *Client) { c.debugOut = w }
}

// Client handles API communication with ScreenScraper
type Client struct {
	httpClient *http.Client
	creds      Credentials
	clock      Clock
	debug      bool
	debugOut   io.Writer
	throttle   time.Duration
	cutoff     *cutoff.Cutoff

	mu          sync.Mutex
	lastRequest time.Time
	requests    int
	quota       Quota

	cacheEnabled bool
	cacheDir     string
	cacheHours   int
	cache        *GameCache
}

// NewClient creates a new ScreenScraper API client
func NewClient(creds Credentials, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		creds:      creds,
		clock:      systemClock{},
		throttle:   defaultThrottle,
		debugOut:   os.Stderr,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.cutoff == nil {
		c.cutoff = cutoff.New()
	}
	if c.cacheEnabled {
		c.cache = NewGameCache(c.cacheDir, c.cacheHours, c.clock.Now)
	}
	return c
}

// Outcome is the answer a lookup produced: a match, not found, or non-game.
type Outcome int

const (
	// OutcomeMatch means ScreenScraper identified the ROM as a game.
	OutcomeMatch Outcome = iota
	// OutcomeNotFound means ScreenScraper has no entry for the ROM.
	OutcomeNotFound
	// OutcomeNonGame means ScreenScraper identified the ROM as something other than a game.
	OutcomeNonGame
)

// String returns the glossary name of the outcome.
func (o Outcome) String() string {
	switch o {
	case OutcomeMatch:
		return "match"
	case OutcomeNotFound:
		return "not found"
	case OutcomeNonGame:
		return "non-game"
	}
	return fmt.Sprintf("Outcome(%d)", int(o))
}

// LookupResult holds the Outcome of a lookup, the Game (set for OutcomeMatch
// and OutcomeNonGame), and whether the answer came from the local cache.
type LookupResult struct {
	Outcome Outcome
	Game    *Game
	Cached  bool
}

func outcomeFor(game *Game) Outcome {
	switch {
	case game == nil:
		return OutcomeNotFound
	case game.IsNonGame():
		return OutcomeNonGame
	default:
		return OutcomeMatch
	}
}

// Lookup asks ScreenScraper (or the local cache of its earlier answers) which
// game a ROM is. Not found and non-game are outcomes, not errors; an error
// means no answer could be obtained.
//
// After a cutoff, any cache entry answers, however old, and a ROM with no
// entry gets a *cutoff.Error without a request being sent.
func (c *Client) Lookup(systemID int, romName string, hashes *ROMHashes) (LookupResult, error) {
	if result, ok := c.cachedLookup(hashes, c.cutoff.Tripped()); ok {
		return result, nil
	}

	params := c.creds.params()
	params.Set("systemeid", fmt.Sprintf("%d", systemID))
	if hashes != nil {
		params.Set("crc", hashes.CRC32)
		params.Set("md5", hashes.MD5)
		params.Set("sha1", hashes.SHA1)
		params.Set("romtaille", fmt.Sprintf("%d", hashes.Size))
	}
	params.Set("romnom", filepath.Base(romName))

	var game *Game
	apiResp, err := getJSONWithRetry[GameInfoJSONResponse](c, "jeuInfos.php", params)
	if err != nil && !isNotFound(err) {
		var ce *cutoff.Error
		if errors.As(err, &ce) {
			if result, ok := c.cachedLookup(hashes, true); ok {
				return result, nil
			}
		}
		return LookupResult{}, fmt.Errorf("failed to look up game: %w", err)
	}
	if err == nil {
		if apiResp.Response.SSUser != nil {
			c.setQuota(apiResp.Response.SSUser)
		}
		game = apiResp.Response.Game
	}

	result := LookupResult{Outcome: outcomeFor(game), Game: game}

	if c.cache != nil && hashes != nil {
		var cacheErr error
		switch result.Outcome {
		case OutcomeNotFound:
			cacheErr = c.cache.SetNotFound(hashes)
		case OutcomeNonGame:
			cacheErr = c.cache.SetNonGame(hashes, game)
		default:
			cacheErr = c.cache.Set(hashes, game)
		}
		if cacheErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cache %s result: %v\n", result.Outcome, cacheErr)
		}
	}

	return result, nil
}

// cachedLookup answers a lookup from the game cache. anyAge accepts expired
// entries.
func (c *Client) cachedLookup(hashes *ROMHashes, anyAge bool) (LookupResult, bool) {
	if c.cache == nil || hashes == nil {
		return LookupResult{}, false
	}
	var cached CacheResult
	if anyAge {
		cached = c.cache.GetAny(hashes)
	} else {
		cached = c.cache.Get(hashes)
	}
	if !cached.Found {
		return LookupResult{}, false
	}
	result := LookupResult{Game: cached.Game, Cached: true}
	switch {
	case cached.NotFound:
		result.Outcome = OutcomeNotFound
	case cached.IsNonGame:
		result.Outcome = OutcomeNonGame
	default:
		result.Outcome = outcomeFor(cached.Game)
	}
	c.debugf("Lookup - CACHED (%s)", result.Outcome)
	return result, true
}

// Quota returns the quota reported by the most recent ScreenScraper response
// that carried one.
func (c *Client) Quota() Quota {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.quota
}

// RequestCount returns the number of HTTP requests this client has sent to
// ScreenScraper, retries included.
func (c *Client) RequestCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

func (c *Client) setQuota(u *UserInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.quota = quotaFrom(u)
}

func (c *Client) debugf(format string, args ...any) {
	if c.debug {
		fmt.Fprintf(c.debugOut, "[DEBUG] API: "+format+"\n", args...)
	}
}

func (c *Client) throttledGet(apiURL string) (*http.Response, error) {
	c.mu.Lock()
	if !c.lastRequest.IsZero() {
		if wait := c.throttle - c.clock.Now().Sub(c.lastRequest); wait > 0 {
			c.clock.Sleep(wait)
		}
	}
	c.lastRequest = c.clock.Now()
	c.requests++
	c.mu.Unlock()

	return c.httpClient.Get(apiURL)
}

func redactedURL(endpoint string, params url.Values) string {
	redacted := url.Values{}
	for k, v := range params {
		redacted[k] = v
	}
	redacted.Set("devpassword", "***")
	redacted.Set("sspassword", "***")
	query := strings.ReplaceAll(redacted.Encode(), "%2A%2A%2A", "***")
	return endpointURL(endpoint, query)
}

func endpointURL(endpoint, query string) string {
	return fmt.Sprintf("%s/%s?%s", apiBaseURL, endpoint, query)
}

// StatusError is a ScreenScraper response with a status that will not change
// on retry, such as bad developer credentials or an exhausted quota.
type StatusError struct {
	StatusCode int
	Body       string
}

// Causes of the permanent statuses that are not cutoff statuses; those are
// named by cutoff.StatusCause.
var permanentStatusCauses = map[int]string{
	400: "ScreenScraper rejected the request as malformed",
	404: "ScreenScraper has no entry for the request",
}

// Error names the cause of the status, with the start of the response body.
func (e *StatusError) Error() string {
	cause, ok := permanentStatusCauses[e.StatusCode]
	if !ok {
		cause = cutoff.StatusCause(e.StatusCode)
	}
	if cause == "" {
		cause = "ScreenScraper returned an unexpected status"
	}
	msg := fmt.Sprintf("%s (%d)", cause, e.StatusCode)
	if body := strings.TrimSpace(e.Body); body != "" {
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		msg += ": " + body
	}
	return msg
}

func isTransientStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusRequestTimeout || status >= 500
}

func isNotFound(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
}

func getJSONWithRetry[T any](c *Client, endpoint string, params url.Values) (*T, error) {
	apiURL := endpointURL(endpoint, params.Encode())
	logURL := redactedURL(endpoint, params)
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			c.clock.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		if cause, ok := c.cutoff.Cause(); ok {
			return nil, &cutoff.Error{Cause: cause}
		}

		c.debugf("GET %s (attempt %d/%d)", logURL, attempt+1, maxAttempts)
		resp, err := c.throttledGet(apiURL)
		if err != nil {
			lastErr = fmt.Errorf("API request failed: %w", err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited by API")
			if attempt < maxAttempts-1 {
				c.clock.Sleep(rateLimitDelay)
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if !isTransientStatus(resp.StatusCode) {
				se := &StatusError{StatusCode: resp.StatusCode, Body: string(body)}
				if cutoff.IsCutoffStatus(resp.StatusCode) {
					return nil, c.cutoff.TripError(resp.StatusCode, se.Body, se)
				}
				return nil, se
			}
			lastErr = fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var v T
		if err := json.Unmarshal(body, &v); err != nil {
			lastErr = fmt.Errorf("failed to decode API response: %w", err)
			continue
		}

		return &v, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}

// GetSystemsList fetches the list of all systems/platforms from ScreenScraper
func (c *Client) GetSystemsList() ([]System, error) {
	apiResp, err := getJSONWithRetry[SystemsListJSONResponse](c, "systemesListe.php", c.creds.params())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch systems list: %w", err)
	}

	if apiResp.Response.SSUser != nil {
		c.setQuota(apiResp.Response.SSUser)
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

// GetUserInfo fetches user account information and API quota details
func (c *Client) GetUserInfo() (*UserInfo, error) {
	apiResp, err := getJSONWithRetry[UserInfoJSONResponse](c, "ssuserInfos.php", c.creds.params())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	if apiResp.Response.SSUser == nil {
		return &UserInfo{}, nil
	}
	c.setQuota(apiResp.Response.SSUser)
	return apiResp.Response.SSUser, nil
}
