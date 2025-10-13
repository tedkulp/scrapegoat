package effects

import (
	"fmt"
	"image"
	"image/color"
)

// OpacityEffect adjusts image opacity
type OpacityEffect struct{}

// Name returns the effect name
func (e *OpacityEffect) Name() string {
	return "opacity"
}

// Validate checks if the effect parameters are valid
func (e *OpacityEffect) Validate(params map[string]interface{}) error {
	if _, ok := params["value"]; !ok {
		return fmt.Errorf("opacity effect requires 'value' parameter (0-100)")
	}
	value := getIntParam(params, "value", 100)
	if value < 0 || value > 100 {
		return fmt.Errorf("opacity value must be between 0 and 100")
	}
	return nil
}

// Apply adjusts the opacity of an image
func (e *OpacityEffect) Apply(img image.Image, params map[string]interface{}) (image.Image, error) {
	value := getIntParam(params, "value", 100)
	if value == 100 {
		return img, nil
	}

	bounds := img.Bounds()
	dst := image.NewNRGBA(bounds)

	// Calculate alpha multiplier (0.0 to 1.0)
	alphaMult := float64(value) / 100.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			c.A = uint8(float64(c.A) * alphaMult)
			dst.SetNRGBA(x, y, c)
		}
	}

	return dst, nil
}
