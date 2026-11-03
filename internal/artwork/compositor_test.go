package artwork

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writePNG writes a small solid PNG and returns its path.
func writePNG(t *testing.T, path string, w, h int) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func screenshotOutput(typ string) OutputDefinition {
	return OutputDefinition{
		Type:   typ,
		Width:  20,
		Height: 10,
		Format: "png",
		Layers: []LayerDefinition{{Resource: "screenshots"}},
	}
}

func TestGenerateArtworkWritesSamePathOnCacheMissAndHit(t *testing.T) {
	mediaRoot := t.TempDir()
	cacheDir := t.TempDir()
	shot := writePNG(t, filepath.Join(mediaRoot, "screenshots", "Game.png"), 8, 8)

	c := NewCompositor(Config{OutputDir: "miximages", Outputs: []OutputDefinition{screenshotOutput("mix")}}, cacheDir)
	media := MediaFiles{"screenshots": shot}
	want := filepath.Join(mediaRoot, "miximages", "Game.png")

	first := c.GenerateArtwork("Game", media, mediaRoot)
	if len(first.Errors) != 0 || len(first.Paths) != 1 || first.Paths[0] != want {
		t.Fatalf("miss: got paths %v errors %v, want [%s]", first.Paths, first.Errors, want)
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}

	second := c.GenerateArtwork("Game", media, mediaRoot)
	if len(second.Errors) != 0 || len(second.Paths) != 1 || second.Paths[0] != want {
		t.Fatalf("hit: got paths %v errors %v, want [%s]", second.Paths, second.Errors, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("artwork not written on cache hit: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(mediaRoot, "miximages"))
	if len(entries) != 1 {
		t.Fatalf("expected exactly one file in artwork folder, got %d", len(entries))
	}
}

func TestGenerateArtworkPerOutputDirOverridesGlobal(t *testing.T) {
	mediaRoot := t.TempDir()
	shot := writePNG(t, filepath.Join(mediaRoot, "screenshots", "Game.png"), 8, 8)

	withOwn := screenshotOutput("marquee")
	withOwn.OutputDir = "marquees"
	c := NewCompositor(Config{OutputDir: "miximages", Outputs: []OutputDefinition{screenshotOutput("mix"), withOwn}}, t.TempDir())

	res := c.GenerateArtwork("Game", MediaFiles{"screenshots": shot}, mediaRoot)
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	want := []string{
		filepath.Join(mediaRoot, "miximages", "Game.png"),
		filepath.Join(mediaRoot, "marquees", "Game.png"),
	}
	if len(res.Paths) != 2 || res.Paths[0] != want[0] || res.Paths[1] != want[1] {
		t.Fatalf("got %v, want %v", res.Paths, want)
	}
}

func TestGenerateArtworkKeepsSuccessfulOutputsWhenOneFails(t *testing.T) {
	mediaRoot := t.TempDir()
	shot := writePNG(t, filepath.Join(mediaRoot, "screenshots", "Game.png"), 8, 8)

	failing := OutputDefinition{
		Type: "boxart", Width: 20, Height: 10, Format: "png", OutputDir: "boxart",
		Layers: []LayerDefinition{{Resource: "box2dfront"}},
	}
	c := NewCompositor(Config{OutputDir: "miximages", Outputs: []OutputDefinition{failing, screenshotOutput("mix")}}, t.TempDir())

	res := c.GenerateArtwork("Game", MediaFiles{"screenshots": shot}, mediaRoot)
	want := filepath.Join(mediaRoot, "miximages", "Game.png")
	if len(res.Paths) != 1 || res.Paths[0] != want {
		t.Fatalf("got paths %v, want [%s]", res.Paths, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("successful output not written: %v", err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error(), "boxart") {
		t.Fatalf("expected one error naming the failed output, got %v", res.Errors)
	}
}

func TestCacheKeyIsStableAndIgnoresUnusedMedia(t *testing.T) {
	dir := t.TempDir()
	media := MediaFiles{
		"screenshots":  writePNG(t, filepath.Join(dir, "screenshots", "Game.png"), 2, 2),
		"box2dfront":   writePNG(t, filepath.Join(dir, "box2dfront", "Game.png"), 2, 2),
		"wheel":        writePNG(t, filepath.Join(dir, "wheel", "Game.png"), 2, 2),
		"titlescreens": writePNG(t, filepath.Join(dir, "titlescreens", "Game.png"), 2, 2),
		"3dboxes":      writePNG(t, filepath.Join(dir, "3dboxes", "Game.png"), 2, 2),
	}
	def := screenshotOutput("mix")
	def.Layers = append(def.Layers, LayerDefinition{Resource: "wheel"}, LayerDefinition{Resource: "box2dfront"})
	c := NewCompositor(Config{}, t.TempDir())

	key := c.calculateCacheKey("Game", def, media)
	for i := 0; i < 50; i++ {
		if got := c.calculateCacheKey("Game", def, media); got != key {
			t.Fatalf("cache key changed on call %d: %s != %s", i, got, key)
		}
	}

	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(media["3dboxes"], later, later); err != nil {
		t.Fatal(err)
	}
	if got := c.calculateCacheKey("Game", def, media); got != key {
		t.Fatalf("cache key changed when unused media changed")
	}

	if err := os.Chtimes(media["wheel"], later, later); err != nil {
		t.Fatal(err)
	}
	if got := c.calculateCacheKey("Game", def, media); got == key {
		t.Fatalf("cache key did not change when used media changed")
	}
}
