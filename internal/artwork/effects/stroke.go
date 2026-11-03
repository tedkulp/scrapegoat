package effects

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/tedkulp/scrapegoat/internal/artwork"
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
	width := artwork.IntParam(params, "width", 1)
	colorStr := artwork.StringParam(params, "color", "#FFFFFF")
	opacity := artwork.IntParam(params, "opacity", 100)

	if width <= 0 {
		return img, nil
	}

	// Parse color
	c, err := artwork.ParseColor(colorStr)
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
