package media

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

func TestPlan_PicksOneFilePerTypeByRegionPreference(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	p := New(cache, "nes", root, 2)
	game := &scraper.Game{Medias: []scraper.Media{
		{Type: "ss", Region: "jp", Format: "png", URL: "http://x/ss-jp"},
		{Type: "ss", Region: "eu", Format: "png", URL: "http://x/ss-eu"},
		{Type: "ss", Region: "wor", Format: "png", URL: "http://x/ss-wor"},
		{Type: "box-2D", Region: "jp", Format: "jpg", URL: "http://x/box-jp"},
		{Type: "box-2D", Region: "eu", Format: "jpg", URL: "http://x/box-eu"},
		{Type: "wheel", Region: "us", Format: "png", URL: "http://x/wheel-us"},
		{Type: "wheel", Region: "wor", Format: "png", URL: "http://x/wheel-wor"},
		{Type: "video", Region: "fr", Format: "mp4", URL: "http://x/video-fr"},
		{Type: "fanart", Region: "us", Format: "jpg", URL: "http://x/fanart"},
		{Type: "manuel", Region: "us", Format: "pdf", URL: ""},
	}}

	plan := p.Plan(game, "Game (USA).zip", Mode{})

	want := map[string]string{
		"screenshots": "http://x/ss-wor",
		"box2dfront":  "http://x/box-eu",
		"wheel":       "http://x/wheel-us",
		"videos":      "http://x/video-fr",
	}
	wantExt := map[string]string{"screenshots": ".png", "box2dfront": ".jpg", "wheel": ".png", "videos": ".mp4"}
	if len(plan.Items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(plan.Items), len(want), plan.Items)
	}
	for _, it := range plan.Items {
		if it.URL != want[it.MediaType] {
			t.Errorf("%s: URL %q, want %q", it.MediaType, it.URL, want[it.MediaType])
		}
		wantPath := filepath.Join(root, it.MediaType, "Game (USA)"+wantExt[it.MediaType])
		if it.Path != wantPath {
			t.Errorf("%s: Path %q, want %q", it.MediaType, it.Path, wantPath)
		}
		wantCache := filepath.Join(cache, "cache", "nes", it.MediaType, "Game (USA)"+wantExt[it.MediaType])
		if it.CachePath != wantCache {
			t.Errorf("%s: CachePath %q, want %q", it.MediaType, it.CachePath, wantCache)
		}
		if it.Source != SourceDownload {
			t.Errorf("%s: Source %v, want download", it.MediaType, it.Source)
		}
	}
}

func TestPlan_MapsEveryScreenScraperType(t *testing.T) {
	p := New(t.TempDir(), "nes", t.TempDir(), 1)
	mapping := map[string]string{
		"box-3D":      "3dboxes",
		"box-2D":      "box2dfront",
		"box-2D-back": "backcovers",
		"manuel":      "manuals",
		"ss":          "screenshots",
		"sstitle":     "titlescreens",
		"wheel":       "wheel",
		"video":       "videos",
		"support-2D":  "physicalmedia",
	}
	game := &scraper.Game{}
	for ssType := range mapping {
		game.Medias = append(game.Medias, scraper.Media{Type: ssType, Region: "us", Format: "png", URL: "http://x/" + ssType})
	}
	plan := p.Plan(game, "g.nes", Mode{})
	if len(plan.Items) != len(mapping) {
		t.Fatalf("got %d items, want %d", len(plan.Items), len(mapping))
	}
	for _, it := range plan.Items {
		ssType := it.URL[len("http://x/"):]
		if mapping[ssType] != it.MediaType {
			t.Errorf("%s mapped to %s, want %s", ssType, it.MediaType, mapping[ssType])
		}
	}
}

// fixture is an httptest ScreenScraper media server plus temp media root and
// cache dirs.
type fixture struct {
	t        *testing.T
	srv      *httptest.Server
	root     string
	cacheDir string
	requests atomic.Int32
	bodies   map[string]string // media param -> body; absent means 404
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{t: t, root: t.TempDir(), cacheDir: t.TempDir(), bodies: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		body, ok := f.bodies[r.URL.Query().Get("media")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fixture) placer() *Placer {
	return New(f.cacheDir, "nes", f.root, 3, WithHTTPClient(f.srv.Client()))
}

// url builds a ScreenScraper-style media URL carrying credentials.
func (f *fixture) url(jeuid, media string) string {
	q := "devid=mydev&devpassword=devsecret&softname=scrapegoat&output=json&ssid=myuser&sspassword=usersecret&systemeid=3"
	if jeuid != "" {
		q += "&jeuid=" + jeuid
	}
	if media != "" {
		q += "&media=" + media
	}
	return f.srv.URL + "/api2/mediaJeu.php?" + q
}

func (f *fixture) rootPath(mediaType, name string) string {
	return filepath.Join(f.root, mediaType, name)
}

func (f *fixture) cachePath(mediaType, name string) string {
	return filepath.Join(f.cacheDir, "cache", "nes", mediaType, name)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists (err=%v), want no file", path, err)
	}
}

func TestExecute_MixedPlanCounts(t *testing.T) {
	f := newFixture(t)
	f.bodies["box-2D(us)"] = "fresh box"
	game := &scraper.Game{Medias: []scraper.Media{
		{Type: "wheel", Region: "us", Format: "png", URL: f.url("42", "wheel(us)")},
		{Type: "ss", Region: "us", Format: "png", URL: f.url("42", "ss(us)")},
		{Type: "box-2D", Region: "us", Format: "png", URL: f.url("42", "box-2D(us)")},
		{Type: "video", Region: "us", Format: "mp4", URL: f.url("42", "video(us)")}, // 404
	}}
	writeFile(t, f.rootPath("wheel", "Game.png"), "existing wheel")
	writeFile(t, f.cachePath("screenshots", "Game.png"), "cached ss")

	p := f.placer()
	plan := p.Plan(game, "Game.nes", Mode{})
	res := p.Execute(plan)

	if res.Existing != 1 || res.FromCache != 1 || res.Downloaded != 1 || res.Failed != 1 {
		t.Errorf("counts existing=%d cache=%d downloaded=%d failed=%d, want 1 each",
			res.Existing, res.FromCache, res.Downloaded, res.Failed)
	}
	if got := f.requests.Load(); got != 2 {
		t.Errorf("HTTP requests = %d, want 2", got)
	}
	wantPlaced := map[string]string{
		"wheel":       f.rootPath("wheel", "Game.png"),
		"screenshots": f.rootPath("screenshots", "Game.png"),
		"box2dfront":  f.rootPath("box2dfront", "Game.png"),
	}
	if len(res.Placed) != len(wantPlaced) {
		t.Errorf("Placed = %v, want %v", res.Placed, wantPlaced)
	}
	for k, v := range wantPlaced {
		if res.Placed[k] != v {
			t.Errorf("Placed[%s] = %q, want %q", k, res.Placed[k], v)
		}
	}
	if _, ok := res.Failures["videos"]; !ok || len(res.Failures) != 1 {
		t.Errorf("Failures = %v, want one for videos", res.Failures)
	}
	if got := readFile(t, f.rootPath("screenshots", "Game.png")); got != "cached ss" {
		t.Errorf("screenshot = %q, want cached ss", got)
	}
	if got := readFile(t, f.rootPath("box2dfront", "Game.png")); got != "fresh box" {
		t.Errorf("box root = %q, want fresh box", got)
	}
	if got := readFile(t, f.cachePath("box2dfront", "Game.png")); got != "fresh box" {
		t.Errorf("box cache = %q, want fresh box", got)
	}
	assertNotExist(t, f.rootPath("videos", "Game.mp4"))
	assertNotExist(t, f.cachePath("videos", "Game.mp4"))
}

func ssGame(url string) *scraper.Game {
	return &scraper.Game{Medias: []scraper.Media{{Type: "ss", Region: "us", Format: "png", URL: url}}}
}

func TestExecute_ForceRefetchesOverMediaRootAndCache(t *testing.T) {
	f := newFixture(t)
	f.bodies["ss(us)"] = "new ss"
	writeFile(t, f.rootPath("screenshots", "Game.png"), "old root")
	writeFile(t, f.cachePath("screenshots", "Game.png"), "old cache")
	writeFile(t, f.cachePath("screenshots", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"ss(us)"}`)

	p := f.placer()
	res := p.Execute(p.Plan(ssGame(f.url("42", "ss(us)")), "Game.nes", Mode{Force: true}))

	if res.Downloaded != 1 || res.Existing != 0 || res.FromCache != 0 {
		t.Errorf("counts %+v, want 1 downloaded", res)
	}
	if got := f.requests.Load(); got != 1 {
		t.Errorf("HTTP requests = %d, want 1", got)
	}
	if got := readFile(t, f.rootPath("screenshots", "Game.png")); got != "new ss" {
		t.Errorf("root = %q, want new ss", got)
	}
	if got := readFile(t, f.cachePath("screenshots", "Game.png")); got != "new ss" {
		t.Errorf("cache = %q, want new ss", got)
	}
}

// snapshot lists every file under dir with its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files[path] = readFile(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPlan_DryRunMatchesRealRunWithoutRequestsOrWrites(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("force=%v", force), func(t *testing.T) {
			f := newFixture(t)
			f.bodies["box-2D(us)"] = "box"
			game := &scraper.Game{Medias: []scraper.Media{
				{Type: "wheel", Region: "us", Format: "png", URL: f.url("42", "wheel(us)")},
				{Type: "ss", Region: "us", Format: "png", URL: f.url("42", "ss(us)")},
				{Type: "sstitle", Region: "us", Format: "png", URL: f.url("42", "sstitle(us)")},
				{Type: "box-2D", Region: "us", Format: "png", URL: f.url("42", "box-2D(us)")},
			}}
			writeFile(t, f.rootPath("wheel", "Game.png"), "wheel")
			writeFile(t, f.cachePath("screenshots", "Game.png"), "ss")
			writeFile(t, f.cachePath("titlescreens", "Game.png"), "title")
			writeFile(t, f.cachePath("titlescreens", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"sstitle(us)"}`)
			before := map[string]map[string]string{"root": snapshot(t, f.root), "cache": snapshot(t, f.cacheDir)}

			p := f.placer()
			dry := p.Plan(game, "Game.nes", Mode{DryRun: true, Force: force})
			dryRes := p.Execute(dry)
			real := p.Plan(game, "Game.nes", Mode{Force: force})

			if !reflect.DeepEqual(dry.Items, real.Items) {
				t.Errorf("dry-run plan %+v\nreal plan %+v", dry.Items, real.Items)
			}
			if got := f.requests.Load(); got != 0 {
				t.Errorf("HTTP requests = %d, want 0", got)
			}
			if dryRes.Downloaded+dryRes.FromCache+dryRes.Existing+dryRes.Failed != 0 {
				t.Errorf("dry-run Execute did work: %+v", dryRes)
			}
			after := map[string]map[string]string{"root": snapshot(t, f.root), "cache": snapshot(t, f.cacheDir)}
			if !reflect.DeepEqual(before, after) {
				t.Errorf("files changed:\nbefore %v\nafter  %v", before, after)
			}

			wantExisting, wantCache, wantDownload := 1, 2, 1
			if force {
				wantExisting, wantCache, wantDownload = 0, 0, 4
			}
			if dry.Count(SourceExisting) != wantExisting || dry.Count(SourceCache) != wantCache || dry.Count(SourceDownload) != wantDownload {
				t.Errorf("plan counts existing=%d cache=%d download=%d, want %d/%d/%d",
					dry.Count(SourceExisting), dry.Count(SourceCache), dry.Count(SourceDownload),
					wantExisting, wantCache, wantDownload)
			}
		})
	}
}

func TestExecute_SidecarMismatchRedownloads(t *testing.T) {
	f := newFixture(t)
	f.bodies["ss(wor)"] = "world ss"
	writeFile(t, f.cachePath("screenshots", "Game.png"), "us ss")
	writeFile(t, f.cachePath("screenshots", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"ss(us)"}`)

	p := f.placer()
	res := p.Execute(p.Plan(ssGame(f.url("42", "ss(wor)")), "Game.nes", Mode{}))

	if res.Downloaded != 1 || f.requests.Load() != 1 {
		t.Errorf("downloaded=%d requests=%d, want 1/1", res.Downloaded, f.requests.Load())
	}
	if got := readFile(t, f.cachePath("screenshots", "Game.png")); got != "world ss" {
		t.Errorf("cache = %q, want world ss", got)
	}
	assertSidecar(t, f.cachePath("screenshots", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"ss(wor)"}`)
}

func assertSidecar(t *testing.T, path, want string) {
	t.Helper()
	var got, exp map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &got); err != nil {
		t.Fatalf("sidecar %s: %v", path, err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("sidecar = %v, want %v", got, exp)
	}
}

func TestExecute_CachedFileWithoutSidecarIsAdopted(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.cachePath("screenshots", "Game.png"), "old ss")

	p := f.placer()
	res := p.Execute(p.Plan(ssGame(f.url("42", "ss(us)")), "Game.nes", Mode{}))

	if res.FromCache != 1 || f.requests.Load() != 0 {
		t.Errorf("fromCache=%d requests=%d, want 1/0", res.FromCache, f.requests.Load())
	}
	if got := readFile(t, f.rootPath("screenshots", "Game.png")); got != "old ss" {
		t.Errorf("root = %q, want old ss", got)
	}
	assertSidecar(t, f.cachePath("screenshots", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"ss(us)"}`)
}

func TestExecute_URLWithoutIdentityUsesCacheAndWritesNoSidecar(t *testing.T) {
	for name, u := range map[string]func(*fixture) string{
		"no jeuid": func(f *fixture) string { return f.url("", "ss(us)") },
		"no media": func(f *fixture) string { return f.url("42", "") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			writeFile(t, f.cachePath("screenshots", "Game.png"), "cached")

			p := f.placer()
			res := p.Execute(p.Plan(ssGame(u(f)), "Game.nes", Mode{}))

			if res.FromCache != 1 || f.requests.Load() != 0 {
				t.Errorf("fromCache=%d requests=%d, want 1/0", res.FromCache, f.requests.Load())
			}
			assertNotExist(t, f.cachePath("screenshots", "Game.png.json"))
		})
	}
}

func TestExecute_ZeroByteCachedFileIsDownloaded(t *testing.T) {
	f := newFixture(t)
	f.bodies["ss(us)"] = "real ss"
	writeFile(t, f.cachePath("screenshots", "Game.png"), "")
	writeFile(t, f.cachePath("screenshots", "Game.png.json"), `{"systemeid":"3","jeuid":"42","media":"ss(us)"}`)

	p := f.placer()
	res := p.Execute(p.Plan(ssGame(f.url("42", "ss(us)")), "Game.nes", Mode{}))

	if res.Downloaded != 1 || f.requests.Load() != 1 {
		t.Errorf("downloaded=%d requests=%d, want 1/1", res.Downloaded, f.requests.Load())
	}
	if got := readFile(t, f.rootPath("screenshots", "Game.png")); got != "real ss" {
		t.Errorf("root = %q, want real ss", got)
	}
}

func TestExecute_BodyFailingMidCopyLeavesNoFile(t *testing.T) {
	root, cacheDir := t.TempDir(), t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "partial data")
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	t.Cleanup(srv.Close)

	p := New(cacheDir, "nes", root, 2, WithHTTPClient(srv.Client()))
	u := srv.URL + "/api2/mediaJeu.php?systemeid=3&jeuid=42&media=ss(us)"
	res := p.Execute(p.Plan(ssGame(u), "Game.nes", Mode{}))

	if res.Failed != 1 || res.Downloaded != 0 {
		t.Errorf("failed=%d downloaded=%d, want 1/0", res.Failed, res.Downloaded)
	}
	cachePath := filepath.Join(cacheDir, "cache", "nes", "screenshots", "Game.png")
	assertNotExist(t, cachePath)
	assertNotExist(t, cachePath+".json")
	assertNotExist(t, filepath.Join(root, "screenshots", "Game.png"))
	if files := snapshot(t, cacheDir); len(files) != 0 {
		t.Errorf("leftover files in cache: %v", files)
	}
}

func TestExecute_SidecarHoldsNoCredentialsOrHost(t *testing.T) {
	f := newFixture(t)
	f.bodies["ss(us)"] = "ss"
	f.bodies["wheel(us)"] = "wheel"
	writeFile(t, f.cachePath("screenshots", "Game.png"), "cached") // adopted
	game := &scraper.Game{Medias: []scraper.Media{
		{Type: "ss", Region: "us", Format: "png", URL: f.url("42", "ss(us)")},
		{Type: "wheel", Region: "us", Format: "png", URL: f.url("42", "wheel(us)")}, // downloaded
	}}

	p := f.placer()
	p.Execute(p.Plan(game, "Game.nes", Mode{}))

	host := strings.TrimPrefix(f.srv.URL, "http://")
	for _, path := range []string{f.cachePath("screenshots", "Game.png.json"), f.cachePath("wheel", "Game.png.json")} {
		content := readFile(t, path)
		for _, banned := range []string{"devpassword", "sspassword", "devid", "ssid", "devsecret", "usersecret", "mydev", "myuser", host, "127.0.0.1"} {
			if strings.Contains(content, banned) {
				t.Errorf("%s contains %q: %s", path, banned, content)
			}
		}
	}
}

func TestExecute_DebugOutputMasksPasswords(t *testing.T) {
	f := newFixture(t)
	f.bodies["ss(us)"] = "ss"
	var buf strings.Builder
	p := New(f.cacheDir, "nes", f.root, 1, WithHTTPClient(f.srv.Client()), WithDebugOutput(&buf))
	p.Execute(p.Plan(ssGame(f.url("42", "ss(us)")), "Game.nes", Mode{}))

	out := buf.String()
	if !strings.Contains(out, "DOWNLOADING") {
		t.Errorf("debug output missing download line: %q", out)
	}
	if strings.Contains(out, "devsecret") || strings.Contains(out, "usersecret") {
		t.Errorf("debug output leaks a password: %q", out)
	}
}
