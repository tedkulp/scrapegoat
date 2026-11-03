package artwork

import (
	"strings"
	"testing"
)

func TestValidateRejectsOutputInMediaTypeFolder(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"global output_dir", Config{OutputDir: "screenshots", Outputs: []OutputDefinition{
			{Type: "mix", Layers: []LayerDefinition{{Resource: "screenshots"}}},
		}}},
		{"per-output output_dir", Config{OutputDir: "miximages", Outputs: []OutputDefinition{
			{Type: "mix", OutputDir: "./Wheel/", Layers: []LayerDefinition{{Resource: "screenshots"}}},
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected an error for an output in a media type folder")
			}
			for _, want := range []string{`"mix"`, "media type folder"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %s", err, want)
				}
			}
		})
	}
}

func TestValidateAcceptsOwnOutputFolder(t *testing.T) {
	cfg := Config{OutputDir: "miximages", Outputs: []OutputDefinition{
		{Type: "mix", Layers: []LayerDefinition{{Resource: "screenshots"}}},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
