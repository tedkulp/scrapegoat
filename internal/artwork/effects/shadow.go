package effects

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/disintegration/imaging"
)

// ShadowEffect adds a drop shadow
type ShadowEffect struct{}

// Name returns the effect name
func (e *ShadowEffect) Name() string {
	return "shadow"
}

// Validate checks if the effect parameters are valid
func (e *ShadowEffect) Validate(params map[string]interface{}) error {
	// All parameters are optional with defaults
	return nil
}

// Apply adds a drop shadow to an image
func (e *ShadowEffect) Apply(img image.Image, params map[string]interface{}) (image.Image, error) {
	distance := getIntParam(params, "distance", 5)
	softness := getFloat64Param(params, "softness", 5.0)
	opacity := getIntParam(params, "opacity", 70)
	angle := getFloat64Param(params, "angle", 135.0) // Default: bottom-right
	colorStr := getStringParam(params, "color", "#000000")

	if distance <= 0 {
		return img, nil
	}

	// Parse shadow color
	shadowColor, err := parseColor(colorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid shadow color: %v", err)
	}

	// Apply opacity to shadow color
	shadowNRGBA := color.NRGBAModel.Convert(shadowColor).(color.NRGBA)
	shadowNRGBA.A = uint8(float64(shadowNRGBA.A) * float64(opacity) / 100.0)

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate shadow offset based on angle (0=right, 90=down, 180=left, 270=up)
	angleRad := angle * math.Pi / 180.0
	offsetX := int(float64(distance) * math.Cos(angleRad))
	offsetY := int(float64(distance) * math.Sin(angleRad))

	// Calculate required canvas size (original + shadow offset + blur radius)
	maxOffset := int(math.Abs(float64(offsetX))) + int(math.Abs(float64(offsetY))) + int(softness)*2
	canvasWidth := width + maxOffset*2
	canvasHeight := height + maxOffset*2

	// Create shadow layer (silhouette of the image)
	shadow := image.NewNRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))

	// Center position for original image
	centerX := maxOffset
	centerY := maxOffset

	// Create shadow silhouette
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_, _, _, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			if a > 0 {
				// Use shadow color with original alpha
				c := shadowNRGBA
				c.A = uint8((uint32(c.A) * a) / 0xffff)
				shadow.SetNRGBA(centerX+x+offsetX, centerY+y+offsetY, c)
			}
		}
	}

	// Apply blur to shadow
	if softness > 0 {
		shadow = imaging.Blur(shadow, softness)
	}

	// Composite original image on top of shadow
	draw.Draw(shadow, image.Rect(centerX, centerY, centerX+width, centerY+height), img, bounds.Min, draw.Over)

	return shadow, nil
}
