package effects

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// StrokeEffect adds a border around an image
type StrokeEffect struct{}

// Name returns the effect name
func (e *StrokeEffect) Name() string {
	return "stroke"
}

// Validate checks if the effect parameters are valid
func (e *StrokeEffect) Validate(params map[string]interface{}) error {
	if _, ok := params["width"]; !ok {
		return fmt.Errorf("stroke effect requires 'width' parameter")
	}
	if _, ok := params["color"]; !ok {
		return fmt.Errorf("stroke effect requires 'color' parameter")
	}
	return nil
}

// Apply adds a border to an image
func (e *StrokeEffect) Apply(img image.Image, params map[string]interface{}) (image.Image, error) {
	width := getIntParam(params, "width", 1)
	colorStr := getStringParam(params, "color", "#FFFFFF")
	opacity := getIntParam(params, "opacity", 100)

	if width <= 0 {
		return img, nil
	}

	// Parse color
	c, err := parseColor(colorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid stroke color: %v", err)
	}

	// Apply opacity to color
	if opacity < 100 {
		nrgba := color.NRGBAModel.Convert(c).(color.NRGBA)
		nrgba.A = uint8(float64(nrgba.A) * float64(opacity) / 100.0)
		c = nrgba
	}

	bounds := img.Bounds()
	oldWidth := bounds.Dx()
	oldHeight := bounds.Dy()

	// Create new image with expanded bounds for the stroke
	newWidth := oldWidth + width*2
	newHeight := oldHeight + width*2
	dst := image.NewNRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Draw the border
	borderColor := image.NewUniform(c)

	// Top
	draw.Draw(dst, image.Rect(0, 0, newWidth, width), borderColor, image.Point{}, draw.Src)
	// Bottom
	draw.Draw(dst, image.Rect(0, newHeight-width, newWidth, newHeight), borderColor, image.Point{}, draw.Src)
	// Left
	draw.Draw(dst, image.Rect(0, width, width, newHeight-width), borderColor, image.Point{}, draw.Src)
	// Right
	draw.Draw(dst, image.Rect(newWidth-width, width, newWidth, newHeight-width), borderColor, image.Point{}, draw.Src)

	// Draw the original image in the center
	draw.Draw(dst, image.Rect(width, width, width+oldWidth, width+oldHeight), img, bounds.Min, draw.Over)

	return dst, nil
}

func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

func parseColor(hex string) (color.Color, error) {
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
