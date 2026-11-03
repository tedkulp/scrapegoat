// Package platform is the platform catalog: every platform Scrapegoat knows,
// from ScreenScraper and from the user's configuration, and the slugs and
// aliases that pick them. It owns fetching the ScreenScraper systems list,
// caching it in <cache-dir>/platforms.json, expiry, slug rules, lookup and
// the listing shown by list-platforms.
package platform

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	"github.com/tedkulp/scrapegoat/internal/screenscraper"
)

// Platform is a game system ScreenScraper catalogs.
type Platform struct {
	ID         int
	Name       string   // the first of Names, e.g. "Atari ST"
	Names      []string // every name ScreenScraper gives it
	Extensions []string // ROM file extensions, with a leading dot
}

// SystemsSource fetches ScreenScraper's systems list. *screenscraper.Client is one.
type SystemsSource interface {
	GetSystemsList() ([]screenscraper.System, error)
}

// Clock tells the catalog the time, for cache expiry.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options builds a Catalog.
type Options struct {
	// Source is asked for the systems list when the cache is missing,
	// expired or in an old format.
	Source SystemsSource
	// Configured are the platforms from config.yaml, keyed by slug. They beat
	// ScreenScraper platforms with the same slug and need no fetch. A Name
	// may be comma-separated like ScreenScraper's; when Names is empty it is
	// filled from Name, and Name becomes the first part.
	Configured map[string]Platform
	// CacheDir holds platforms.json.
	CacheDir string
	// Expiry is how long platforms.json stays fresh.
	Expiry time.Duration
	// Clock defaults to the system clock.
	Clock Clock
}

// Entry is one row of the listing.
type Entry struct {
	Platform Platform
	// Slug is the platform's alias if it has one, otherwise its shortest
	// slug (ties alphabetical) among those that pick it. Empty when every
	// slug it produces picks another platform.
	Slug string
	// FromConfig marks a platform defined in config.yaml.
	FromConfig bool
}

// Catalog resolves platform slugs. Its ScreenScraper part is loaded on first
// use, from the cache or the source.
type Catalog struct {
	opts       Options
	configured map[string]Platform // lowercased slug -> platform

	once      sync.Once
	loadErr   error
	platforms []Platform     // ScreenScraper platforms, by ID
	slugs     map[string]int // slug -> index into platforms
}

// New builds a catalog. Nothing is read or fetched until Lookup or List.
func New(opts Options) *Catalog {
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	configured := make(map[string]Platform, len(opts.Configured))
	for s, p := range opts.Configured {
		configured[strings.ToLower(s)] = configuredPlatform(p)
	}
	return &Catalog{opts: opts, configured: configured}
}

// Lookup returns the platform a slug picks, ignoring case. Platforms from
// config.yaml are checked first, without touching the cache or the network.
func (c *Catalog) Lookup(platformSlug string) (Platform, error) {
	key := strings.ToLower(platformSlug)
	if p, ok := c.configured[key]; ok {
		return p, nil
	}
	if err := c.load(); err != nil {
		return Platform{}, fmt.Errorf("platform %s not found in configuration and failed to fetch from API: %w", platformSlug, err)
	}
	i, ok := c.slugs[key]
	if !ok {
		return Platform{}, fmt.Errorf("platform %s not found in configuration or API", platformSlug)
	}
	return c.platforms[i], nil
}

// List returns every platform: config.yaml platforms first (by slug), then
// ScreenScraper platforms by ID.
func (c *Catalog) List() ([]Entry, error) {
	if err := c.load(); err != nil {
		return nil, err
	}

	var entries []Entry
	configSlugs := make([]string, 0, len(c.configured))
	for s := range c.configured {
		configSlugs = append(configSlugs, s)
	}
	sort.Strings(configSlugs)
	for _, s := range configSlugs {
		entries = append(entries, Entry{Platform: c.configured[s], Slug: s, FromConfig: true})
	}

	won := make([][]string, len(c.platforms))
	for s, i := range c.slugs {
		if _, taken := c.configured[s]; taken {
			continue
		}
		won[i] = append(won[i], s)
	}
	for i, p := range c.platforms {
		entries = append(entries, Entry{Platform: p, Slug: displaySlug(p.ID, won[i])})
	}
	return entries, nil
}

// displaySlug picks the slug list-platforms shows from those a platform won.
func displaySlug(id int, won []string) string {
	if a, ok := aliasFor(id); ok {
		for _, s := range won {
			if s == a {
				return a
			}
		}
	}
	best := ""
	for _, s := range won {
		if best == "" || len(s) < len(best) || (len(s) == len(best) && s < best) {
			best = s
		}
	}
	return best
}

func (c *Catalog) load() error {
	c.once.Do(func() {
		systems, err := c.systems()
		if err != nil {
			c.loadErr = err
			return
		}
		c.build(systems)
	})
	return c.loadErr
}

// build applies the slug rules: one slug per name part, collisions to the
// lowest ID, then aliases over name slugs. Configured platforms are applied
// at lookup time and beat all of these.
func (c *Catalog) build(systems []screenscraper.System) {
	regions := screenscraper.DefaultRegions()
	c.platforms = nil
	for _, s := range systems {
		names := splitNames(s.GetPreferredName(regions))
		if len(names) == 0 {
			continue
		}
		c.platforms = append(c.platforms, Platform{
			ID:         s.ID,
			Name:       names[0],
			Names:      names,
			Extensions: s.Extensions,
		})
	}
	sort.SliceStable(c.platforms, func(a, b int) bool { return c.platforms[a].ID < c.platforms[b].ID })

	c.slugs = make(map[string]int)
	byID := make(map[int]int, len(c.platforms))
	for i, p := range c.platforms {
		if _, dup := byID[p.ID]; !dup {
			byID[p.ID] = i
		}
		for _, n := range p.Names {
			s := nameSlug(n)
			if s == "" {
				continue
			}
			if _, taken := c.slugs[s]; !taken {
				c.slugs[s] = i
			}
		}
	}
	for _, a := range aliases {
		if i, ok := byID[a.id]; ok {
			c.slugs[a.slug] = i
		}
	}
}

// configuredPlatform fills Names from a config.yaml platform's Name.
func configuredPlatform(p Platform) Platform {
	if len(p.Names) == 0 {
		p.Names = splitNames(p.Name)
	}
	if len(p.Names) > 0 {
		p.Name = p.Names[0]
	}
	return p
}

// splitNames splits ScreenScraper's comma-separated name into its parts.
func splitNames(full string) []string {
	var names []string
	for _, part := range strings.Split(full, ",") {
		if part = strings.TrimSpace(part); part != "" {
			names = append(names, part)
		}
	}
	return names
}

// nameSlug makes a slug from one name: lowercased, URL-safe (², é and the
// like transliterated) and with dashes removed, so "N64 DD" gives "n64dd".
func nameSlug(name string) string {
	return strings.ReplaceAll(slug.Make(name), "-", "")
}
