package artwork

import (
	"image"
	"testing"
)

func TestCalculateLayerSize(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 100)) // 2:1 source
	tests := []struct {
		name          string
		width, height string
		wantW, wantH  int
	}{
		{"unset keeps source size", "", "", 200, 100},
		{"pixels", "50", "40", 50, 40},
		{"percent of output", "50%", "25%", 320, 120},
		{"auto width keeps aspect ratio", "auto", "60", 120, 60},
		{"auto height keeps aspect ratio", "300", "auto", 300, 150},
		{"both auto keeps source size", "auto", "auto", 200, 100},
		{"negative pixels fall back to source size", "-10", "-5", 200, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h, err := calculateLayerSize(LayerDefinition{Width: tt.width, Height: tt.height}, img, 640, 480)
			if err != nil {
				t.Fatal(err)
			}
			if w != tt.wantW || h != tt.wantH {
				t.Fatalf("got %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestCalculateLayerSizeRejectsGarbage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if _, _, err := calculateLayerSize(LayerDefinition{Width: "wide"}, img, 640, 480); err == nil {
		t.Fatal("expected an error for a non-numeric width")
	}
}

func TestCalculateLayerPosition(t *testing.T) {
	// 100x50 layer on a 640x480 output.
	tests := []struct {
		name         string
		def          LayerDefinition
		wantX, wantY int
	}{
		{"default is top-left", LayerDefinition{}, 0, 0},
		{"pixel offsets from top-left", LayerDefinition{X: "10", Y: "20"}, 10, 20},
		{"percent offsets", LayerDefinition{X: "10%", Y: "50%"}, 64, 240},
		{"center and middle", LayerDefinition{Align: "center", Valign: "middle"}, 270, 215},
		{"center with offset", LayerDefinition{Align: "center", Valign: "middle", X: "5", Y: "-5"}, 275, 210},
		{"negative offsets from bottom-right", LayerDefinition{Align: "right", Valign: "bottom", X: "-20", Y: "-20"}, 520, 410},
		{"negative percent from bottom-right", LayerDefinition{Align: "right", Valign: "bottom", X: "-10%", Y: "-10%"}, 476, 382},
		{"negative offset from top-left goes off canvas", LayerDefinition{X: "-30", Y: "-10"}, -30, -10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, err := calculateLayerPosition(tt.def, 100, 50, 640, 480)
			if err != nil {
				t.Fatal(err)
			}
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("got (%d,%d), want (%d,%d)", x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestCalculateLayerPositionRejectsBadAlignment(t *testing.T) {
	if _, _, err := calculateLayerPosition(LayerDefinition{Align: "sideways"}, 1, 1, 10, 10); err == nil {
		t.Fatal("expected an error for an invalid align")
	}
	if _, _, err := calculateLayerPosition(LayerDefinition{Valign: "sideways"}, 1, 1, 10, 10); err == nil {
		t.Fatal("expected an error for an invalid valign")
	}
}
