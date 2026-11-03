// Package cutoff records a run's cutoff: ScreenScraper refusing every further
// request because the quota or failed-request quota is used up, the developer
// credentials are rejected, the software is blacklisted or the API is closed.
// One Cutoff is shared by everything in a run that talks to ScreenScraper, so
// that after the first cutoff status nothing sends another request.
package cutoff

import (
	"fmt"
	"strings"
	"sync"
)

var causes = map[int]string{
	403: "ScreenScraper rejected developer credentials",
	423: "ScreenScraper API is closed",
	426: "ScreenScraper has blacklisted this software name; update Scrapegoat",
	430: "ScreenScraper daily quota exceeded",
	431: "ScreenScraper failed-request quota exceeded (too many unrecognized ROMs today)",
}

// IsCutoffStatus reports whether a ScreenScraper response status cuts the run
// off: 403, 423, 426, 430 or 431.
func IsCutoffStatus(status int) bool {
	_, ok := causes[status]
	return ok
}

// StatusCause names what a cutoff status means, or "" for any other status.
func StatusCause(status int) string { return causes[status] }

// Cause is the response that cut the run off.
type Cause struct {
	StatusCode int
	// Body is the start of the response body, as ScreenScraper sent it.
	Body string
}

// String names the cause and its status, for example
// "ScreenScraper daily quota exceeded (430)".
func (c Cause) String() string {
	name := StatusCause(c.StatusCode)
	if name == "" {
		name = "ScreenScraper refused further requests"
	}
	return fmt.Sprintf("%s (%d)", name, c.StatusCode)
}

// BodyExcerpt is the start of a response body for an error message: trimmed,
// and cut to 200 bytes with "..." when longer.
func BodyExcerpt(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	return body
}

// Cutoff is a run's cutoff state. It is safe for concurrent use; the zero
// value has not been tripped.
type Cutoff struct {
	mu    sync.Mutex
	cause *Cause
}

// New returns a Cutoff that has not been tripped.
func New() *Cutoff { return &Cutoff{} }

// Trip records a cutoff caused by a response with status and body. Only the
// first call records its cause; Trip reports whether this call was it.
func (c *Cutoff) Trip(status int, body string) bool {
	body = BodyExcerpt(body)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cause != nil {
		return false
	}
	c.cause = &Cause{StatusCode: status, Body: body}
	return true
}

// TripError trips the cutoff for a response with status and body and returns
// the error for that response: a *Error carrying the run's first cause and
// wrapping err.
func (c *Cutoff) TripError(status int, body string, err error) *Error {
	c.Trip(status, body)
	cause, _ := c.Cause()
	return &Error{Cause: cause, Err: err}
}

// Tripped reports whether the run has been cut off.
func (c *Cutoff) Tripped() bool {
	_, ok := c.Cause()
	return ok
}

// Cause returns the first cause recorded, and whether there is one.
func (c *Cutoff) Cause() (Cause, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cause == nil {
		return Cause{}, false
	}
	return *c.cause, true
}

// Error is the error for work that was not done because of a cutoff. Err is
// the response error when this request is the one that tripped the cutoff,
// and nil when the cutoff was already in place and no request was sent.
type Error struct {
	Cause Cause
	Err   error
}

func (e *Error) Error() string {
	msg := "cut off by ScreenScraper: " + e.Cause.String()
	if e.Cause.Body != "" {
		msg += ": " + e.Cause.Body
	}
	return msg
}

// Unwrap returns the response error, if any.
func (e *Error) Unwrap() error { return e.Err }
