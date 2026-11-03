// Package media places a game's media files for EmulationStation: it picks one
// file per media type, decides whether each comes from the media root, the
// media cache or ScreenScraper, and writes it into place.
package media

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

var typeMapping = []struct{ screenScraper, mediaType string }{
	{"box-3D", "3dboxes"},
	{"box-2D", "box2dfront"},
	{"box-2D-back", "backcovers"},
	{"manuel", "manuals"},
	{"ss", "screenshots"},
	{"sstitle", "titlescreens"},
	{"wheel", "wheel"},
	{"video", "videos"},
	{"support-2D", "physicalmedia"},
}

// Types returns the EmulationStation media type folders media is placed in,
// in plan order. Each call returns a new slice.
func Types() []string {
	types := make([]string, len(typeMapping))
	for i, m := range typeMapping {
		types[i] = m.mediaType
	}
	return types
}

// Placer plans and places media files for one platform and media root.
type Placer struct {
	cacheDir   string
	mediaRoot  string
	workers    int
	httpClient *http.Client
	debug      io.Writer
	debugMu    sync.Mutex
}

// Option configures a Placer.
type Option func(*Placer)

// WithHTTPClient sets the HTTP client used to fetch media.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Placer) { p.httpClient = c }
}

// WithDebugOutput sends debug logging to w. Without it the Placer logs nothing.
func WithDebugOutput(w io.Writer) Option {
	return func(p *Placer) { p.debug = w }
}

// New returns a Placer that caches media under <cacheDir>/cache/<platform> and
// places it under mediaRoot, fetching with up to workers concurrent requests.
// It touches neither the filesystem nor the network.
func New(cacheDir, platform, mediaRoot string, workers int, opts ...Option) *Placer {
	if workers < 1 {
		workers = 1
	}
	p := &Placer{
		cacheDir:   filepath.Join(cacheDir, "cache", platform),
		mediaRoot:  mediaRoot,
		workers:    workers,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		debug:      io.Discard,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Root returns the media root the Placer places media under.
func (p *Placer) Root() string { return p.mediaRoot }

// Mode says how a plan is built.
type Mode struct {
	// DryRun marks a plan that will only be reported; Execute refuses it.
	DryRun bool
	// Force fetches every selected file from ScreenScraper again, skipping
	// both the media root and the media cache.
	Force bool
}

// Source says where a planned media file comes from.
type Source int

const (
	// SourceExisting: the file is already in the media root.
	SourceExisting Source = iota
	// SourceCache: the file is copied from the media cache.
	SourceCache
	// SourceDownload: the file is fetched from ScreenScraper.
	SourceDownload
)

// String names the source for logs.
func (s Source) String() string {
	switch s {
	case SourceExisting:
		return "existing"
	case SourceCache:
		return "cache"
	case SourceDownload:
		return "download"
	}
	return "unknown"
}

// Item is one planned media file.
type Item struct {
	MediaType string // EmulationStation media type folder, e.g. "box2dfront"
	URL       string
	Path      string // where the file goes in the media root
	CachePath string
	Source    Source

	identity    identity
	hasIdentity bool
	adopt       bool
}

// Plan is a ROM's media plan.
type Plan struct {
	Items  []Item
	dryRun bool
}

// Count returns how many planned files come from source s.
func (pl Plan) Count(s Source) int {
	n := 0
	for _, it := range pl.Items {
		if it.Source == s {
			n++
		}
	}
	return n
}

// Plan builds the media plan for a ROM. It makes no network requests and
// writes nothing, so a dry run and a real run get the same plan.
func (p *Placer) Plan(game *scraper.Game, romFilename string, mode Mode) Plan {
	romName := strings.TrimSuffix(romFilename, filepath.Ext(romFilename))
	plan := Plan{dryRun: mode.DryRun}
	for _, m := range typeMapping {
		media := game.GetPreferredMedia(m.screenScraper, scraper.DefaultRegions())
		if media == nil || media.URL == "" {
			continue
		}
		ext := media.Format
		if ext == "" {
			ext = filepath.Ext(media.URL)
		}
		if ext != "" && !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		filename := romName + ext
		it := Item{
			MediaType: m.mediaType,
			URL:       media.URL,
			Path:      filepath.Join(p.mediaRoot, m.mediaType, filename),
			CachePath: filepath.Join(p.cacheDir, m.mediaType, filename),
			Source:    SourceDownload,
		}
		it.identity, it.hasIdentity = identityFromURL(media.URL)
		if !mode.Force {
			p.chooseSource(&it)
		}
		plan.Items = append(plan.Items, it)
	}
	return plan
}

func (p *Placer) chooseSource(it *Item) {
	if _, err := os.Stat(it.Path); err == nil {
		it.Source = SourceExisting
		return
	}
	info, err := os.Stat(it.CachePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return
	}
	if !it.hasIdentity {
		it.Source = SourceCache
		return
	}
	stored, err := readSidecar(it.CachePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		it.Source = SourceCache
		it.adopt = true
	case err == nil && stored == it.identity:
		it.Source = SourceCache
	}
}

type identity struct {
	SystemeID string `json:"systemeid"`
	JeuID     string `json:"jeuid"`
	Media     string `json:"media"`
}

func identityFromURL(raw string) (identity, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return identity{}, false
	}
	q := u.Query()
	id := identity{SystemeID: q.Get("systemeid"), JeuID: q.Get("jeuid"), Media: q.Get("media")}
	return id, id.JeuID != "" && id.Media != ""
}

func sidecarPath(cachePath string) string { return cachePath + ".json" }

func readSidecar(cachePath string) (identity, error) {
	b, err := os.ReadFile(sidecarPath(cachePath))
	if err != nil {
		return identity{}, err
	}
	var id identity
	if err := json.Unmarshal(b, &id); err != nil {
		return identity{}, err
	}
	return id, nil
}

func writeSidecar(cachePath string, id identity) error {
	b, err := json.Marshal(id)
	if err != nil {
		return err
	}
	return writeAtomic(sidecarPath(cachePath), bytes.NewReader(b))
}

// Result is what placing one ROM's media produced.
type Result struct {
	// Placed maps each media type to its file in the media root.
	Placed     map[string]string
	Existing   int
	FromCache  int
	Downloaded int
	Failed     int
	// Failures maps each failed media type to its error.
	Failures map[string]error
}

// Execute carries out a plan, fetching files concurrently with the Placer's
// workers. A dry-run plan is not carried out: Execute writes nothing and
// returns an empty result.
func (p *Placer) Execute(plan Plan) Result {
	res := Result{Placed: map[string]string{}, Failures: map[string]error{}}
	if plan.dryRun {
		return res
	}

	var work []Item
	for _, it := range plan.Items {
		if it.Source == SourceExisting {
			p.debugf("MEDIA: %s - EXISTING %s", it.MediaType, it.Path)
			res.Placed[it.MediaType] = it.Path
			res.Existing++
			continue
		}
		work = append(work, it)
	}

	type outcome struct {
		item Item
		err  error
	}
	jobs := make(chan Item)
	outcomes := make(chan outcome, len(work))
	var wg sync.WaitGroup
	for i := 0; i < min(p.workers, len(work)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range jobs {
				outcomes <- outcome{it, p.place(it)}
			}
		}()
	}
	for _, it := range work {
		jobs <- it
	}
	close(jobs)
	wg.Wait()
	close(outcomes)

	for o := range outcomes {
		switch {
		case o.err != nil:
			res.Failed++
			res.Failures[o.item.MediaType] = o.err
		case o.item.Source == SourceCache:
			res.FromCache++
			res.Placed[o.item.MediaType] = o.item.Path
		default:
			res.Downloaded++
			res.Placed[o.item.MediaType] = o.item.Path
		}
	}
	return res
}

func (p *Placer) place(it Item) error {
	if it.Source == SourceDownload {
		if err := p.fetch(it); err != nil {
			return err
		}
	} else {
		p.debugf("MEDIA: %s - CACHED (no API call)", it.MediaType)
		if it.adopt {
			if err := writeSidecar(it.CachePath, it.identity); err != nil {
				p.debugf("MEDIA: %s - failed to write sidecar: %v", it.MediaType, err)
			}
		}
	}
	src, err := os.Open(it.CachePath)
	if err != nil {
		return fmt.Errorf("open cached %s: %w", it.MediaType, err)
	}
	defer src.Close()
	if err := writeAtomic(it.Path, src); err != nil {
		return fmt.Errorf("place %s: %w", it.MediaType, err)
	}
	return nil
}

func (p *Placer) fetch(it Item) error {
	p.debugf("MEDIA: %s - DOWNLOADING (API call) %s", it.MediaType, redactURL(it.URL))
	resp, err := p.httpClient.Get(it.URL)
	if err != nil {
		return fmt.Errorf("download %s: %w", it.MediaType, redactErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP status %d", it.MediaType, resp.StatusCode)
	}
	if err := writeAtomic(it.CachePath, resp.Body); err != nil {
		return fmt.Errorf("download %s: %w", it.MediaType, err)
	}
	if it.hasIdentity {
		err = writeSidecar(it.CachePath, it.identity)
	} else {
		// A sidecar left from an earlier download no longer describes this file.
		err = os.Remove(sidecarPath(it.CachePath))
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	}
	if err != nil {
		p.debugf("MEDIA: %s - failed to update sidecar: %v", it.MediaType, err)
	}
	return nil
}

func writeAtomic(path string, r io.Reader) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if _, err = io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparseable URL>"
	}
	q := u.Query()
	for _, k := range []string{"devpassword", "sspassword"} {
		if q.Has(k) {
			q.Set(k, "***")
		}
	}
	u.RawQuery = strings.ReplaceAll(q.Encode(), "%2A%2A%2A", "***")
	return u.String()
}

func redactErr(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return fmt.Errorf("%s: %w", uerr.Op, uerr.Err)
	}
	return err
}

func (p *Placer) debugf(format string, args ...any) {
	p.debugMu.Lock()
	defer p.debugMu.Unlock()
	fmt.Fprintf(p.debug, "[DEBUG] "+format+"\n", args...)
}
