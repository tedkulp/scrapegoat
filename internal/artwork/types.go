package artwork

import (
	"fmt"
	"image"
	"image/color"

	"go.yaml.in/yaml/v3"
)

// Config represents the artwork generation configuration
type Config struct {
	Enabled      bool               `yaml:"enabled"`
	OutputDir    string             `yaml:"output_dir"`
	ResourcesDir string             `yaml:"resources_dir"`
	Outputs      []OutputDefinition `yaml:"outputs"`
}

// OutputDefinition defines a single output image to generate
type OutputDefinition struct {
	Type       string            `yaml:"type"`        // Output type (e.g., "screenshot", "marquee")
	Width      int               `yaml:"width"`       // Target width in pixels (0 = auto)
	Height     int               `yaml:"height"`      // Target height in pixels (0 = auto)
	Format     string            `yaml:"format"`      // Output format: "png", "jpg", "webp"
	Background string            `yaml:"background"`  // Background color (hex) or "transparent"
	Layers     []LayerDefinition `yaml:"layers"`      // Layers to composite (bottom to top)
}

// LayerDefinition defines a single layer in the composition
type LayerDefinition struct {
	Resource string                   `yaml:"resource"` // Source media type or "custom:{path}"
	X        string                   `yaml:"x"`        // X position (pixels, %, or negative)
	Y        string                   `yaml:"y"`        // Y position
	Width    string                   `yaml:"width"`    // Width (pixels, %, or "auto")
	Height   string                   `yaml:"height"`   // Height
	Align    string                   `yaml:"align"`    // Horizontal align: left, center, right
	Valign   string                   `yaml:"valign"`   // Vertical align: top, middle, bottom
	Effects  []EffectDefinition       `yaml:"effects"`  // Effects to apply (in order)
}

// EffectDefinition defines an effect to apply to a layer
type EffectDefinition struct {
	Type   string                 `yaml:"type"`   // Effect type name
	Params map[string]interface{} // Effect parameters (populated via UnmarshalYAML)
}

// UnmarshalYAML implements custom YAML unmarshaling for EffectDefinition
// This captures the "type" field separately and puts all other fields into Params
func (e *EffectDefinition) UnmarshalYAML(node *yaml.Node) error {
	// First, unmarshal into a temporary map to get all fields
	var temp map[string]interface{}
	if err := node.Decode(&temp); err != nil {
		return err
	}

	// Extract the type field
	typeVal, ok := temp["type"]
	if !ok {
		return fmt.Errorf("effect definition missing 'type' field")
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return fmt.Errorf("effect 'type' must be a string")
	}

	e.Type = typeStr

	// Everything else goes into Params
	e.Params = make(map[string]interface{})
	for key, val := range temp {
		if key != "type" {
			e.Params[key] = val
		}
	}

	return nil
}

// Effect is the interface that all effects must implement
type Effect interface {
	// Apply the effect to an image
	Apply(img image.Image, params map[string]interface{}) (image.Image, error)

	// Validate effect parameters
	Validate(params map[string]interface{}) error

	// Get effect name
	Name() string
}

// Layer represents a processed layer ready for composition
type Layer struct {
	Image  image.Image
	X      int
	Y      int
	Width  int
	Height int
}

// MediaFiles maps media types to file paths
type MediaFiles map[string]string

// ParseColor parses a hex color string to color.Color
func ParseColor(hex string) (color.Color, error) {
	if hex == "" || hex == "transparent" {
		return color.Transparent, nil
	}

	// Remove # prefix if present
	if hex[0] == '#' {
		hex = hex[1:]
	}

	// Parse RGB or RGBA
	var r, g, b, a uint8
	a = 255 // Default to opaque

	switch len(hex) {
	case 6: // RGB
		_, err := parseHexByte(hex[0:2], &r)
		if err != nil {
			return nil, err
		}
		_, err = parseHexByte(hex[2:4], &g)
		if err != nil {
			return nil, err
		}
		_, err = parseHexByte(hex[4:6], &b)
		if err != nil {
			return nil, err
		}
	case 8: // RGBA
		_, err := parseHexByte(hex[0:2], &r)
		if err != nil {
			return nil, err
		}
		_, err = parseHexByte(hex[2:4], &g)
		if err != nil {
			return nil, err
		}
		_, err = parseHexByte(hex[4:6], &b)
		if err != nil {
			return nil, err
		}
		_, err = parseHexByte(hex[6:8], &a)
		if err != nil {
			return nil, err
		}
	default:
		return nil, image.ErrFormat
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}, nil
}

func parseHexByte(s string, out *uint8) (int, error) {
	var v uint8
	for i := 0; i < len(s); i++ {
		c := s[i]
		var d uint8
		switch {
		case '0' <= c && c <= '9':
			d = c - '0'
		case 'a' <= c && c <= 'f':
			d = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			d = c - 'A' + 10
		default:
			return 0, image.ErrFormat
		}
		v = v*16 + d
	}
	*out = v
	return 2, nil
}
