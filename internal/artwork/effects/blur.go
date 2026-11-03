package effects

import (
	"fmt"
	"image"

	"github.com/disintegration/imaging"
	"github.com/tedkulp/scrapegoat/internal/artwork"
)

// BlurEffect applies Gaussian blur
type BlurEffect struct{}

// Name returns the effect name
func (e *BlurEffect) Name() string {
	return "blur"
}

// Validate checks if the effect parameters are valid
func (e *BlurEffect) Validate(params map[string]interface{}) error {
	if _, ok := params["radius"]; !ok {
		return fmt.Errorf("blur effect requires 'radius' parameter")
	}
	return nil
}

// Apply applies Gaussian blur to an image
func (e *BlurEffect) Apply(img image.Image, params map[string]interface{}) (image.Image, error) {
	radius := artwork.Float64Param(params, "radius", 5.0)
	if radius <= 0 {
		return img, nil
	}

	return imaging.Blur(img, radius), nil
}
