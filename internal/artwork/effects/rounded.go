package effects

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

// RoundedEffect implements rounded corners
type RoundedEffect struct{}

func init() {
	// This will be called when the effects package is imported
}

// Name returns the effect name
func (e *RoundedEffect) Name() string {
	return "rounded"
}

// Validate checks if the effect parameters are valid
func (e *RoundedEffect) Validate(params map[string]interface{}) error {
	if _, ok := params["radius"]; !ok {
		return fmt.Errorf("rounded effect requires 'radius' parameter")
	}
	return nil
}

// Apply applies rounded corners to an image
func (e *RoundedEffect) Apply(img image.Image, params map[string]interface{}) (image.Image, error) {
	radius := getIntParam(params, "radius", 10)
	if radius <= 0 {
		return img, nil
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create a new NRGBA image with transparency
	dst := image.NewNRGBA(bounds)

	// Create a mask for rounded corners
	mask := image.NewAlpha(bounds)

	// Fill the mask with opaque (white)
	opaque := color.Alpha{A: 255}
	transparent := color.Alpha{A: 0}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mask.SetAlpha(x, y, opaque)
		}
	}

	// Apply rounded corners by making corners transparent
	// Top-left corner
	for y := 0; y < radius; y++ {
		for x := 0; x < radius; x++ {
			dx := radius - x
			dy := radius - y
			if dx*dx+dy*dy > radius*radius {
				mask.SetAlpha(x, y, transparent)
			}
		}
	}

	// Top-right corner
	for y := 0; y < radius; y++ {
		for x := width - radius; x < width; x++ {
			dx := x - (width - radius)
			dy := radius - y
			if dx*dx+dy*dy > radius*radius {
				mask.SetAlpha(x, y, transparent)
			}
		}
	}

	// Bottom-left corner
	for y := height - radius; y < height; y++ {
		for x := 0; x < radius; x++ {
			dx := radius - x
			dy := y - (height - radius)
			if dx*dx+dy*dy > radius*radius {
				mask.SetAlpha(x, y, transparent)
			}
		}
	}

	// Bottom-right corner
	for y := height - radius; y < height; y++ {
		for x := width - radius; x < width; x++ {
			dx := x - (width - radius)
			dy := y - (height - radius)
			if dx*dx+dy*dy > radius*radius {
				mask.SetAlpha(x, y, transparent)
			}
		}
	}

	// Apply mask to the image
	draw.DrawMask(dst, bounds, img, image.Point{}, mask, image.Point{}, draw.Over)

	return dst, nil
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultValue
}
