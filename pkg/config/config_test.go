package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const credentialsYAML = `
screenscraper:
  dev_id: dev
  dev_password: devpw
  user_id: user
  user_password: userpw
`

func loadYAML(t *testing.T, artworkYAML string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(credentialsYAML+artworkYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestAcceptsMediaTypeAndCustomResources(t *testing.T) {
	cfg, err := loadArtwork(t, `
artwork:
  output_dir: miximages
  outputs:
    - type: mix
      format: png
      layers:
        - resource: screenshots
        - resource: box2dfront
        - resource: "custom:frames/tv.png"
    - type: marquee
      format: png
      output_dir: marquees
      layers:
        - resource: wheel
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Artwork.Outputs[1].OutputDir; got != "marquees" {
		t.Fatalf("per-output output_dir = %q, want marquees", got)
	}
}

// The commented-out artwork section of config.yaml.example must be a valid
// configuration once uncommented.
func TestExampleArtworkConfigLoads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	example := string(data)
	start := strings.Index(example, "# artwork:")
	if start < 0 {
		t.Fatal("config.yaml.example has no artwork section")
	}
	var section strings.Builder
	for _, line := range strings.Split(example[start:], "\n") {
		section.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "#"), " "))
		section.WriteString("\n")
	}

	cfg, err := loadArtwork(t, section.String())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Artwork.Outputs) < 2 {
		t.Fatalf("expected the example's artwork outputs to load, got %d", len(cfg.Artwork.Outputs))
	}
}

// loadArtwork loads the config and validates its artwork section, as a scrape
// with artwork enabled does.
func loadArtwork(t *testing.T, artworkYAML string) (*Config, error) {
	t.Helper()
	cfg, err := loadYAML(t, artworkYAML)
	if err != nil {
		return nil, err
	}
	return cfg, cfg.Artwork.Validate()
}

func TestLoadDoesNotValidateArtwork(t *testing.T) {
	_, err := loadYAML(t, `
artwork:
  enabled: false
  outputs:
    - type: mix
      layers:
        - resource: cover
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestRejectsOutputsWithSameFolderAndFormat(t *testing.T) {
	_, err := loadArtwork(t, `
artwork:
  output_dir: miximages
  outputs:
    - type: first
      format: png
      layers:
        - resource: screenshots
    - type: second
      format: png
      output_dir: miximages
      layers:
        - resource: wheel
`)
	if err == nil {
		t.Fatal("expected an error for colliding artwork outputs")
	}
	for _, name := range []string{"first", "second"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name output %q", err, name)
		}
	}
}

func TestAcceptsSameFolderWithDifferentFormats(t *testing.T) {
	_, err := loadArtwork(t, `
artwork:
  outputs:
    - type: first
      format: png
      layers:
        - resource: screenshots
    - type: second
      format: jpg
      layers:
        - resource: screenshots
`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestRejectsUnknownLayerResource(t *testing.T) {
	_, err := loadArtwork(t, `
artwork:
  outputs:
    - type: mix
      layers:
        - resource: screenshots
        - resource: cover
`)
	if err == nil {
		t.Fatal("expected an error for an unknown layer resource")
	}
	for _, want := range []string{"mix", "cover"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
