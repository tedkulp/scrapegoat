package artwork

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
)

// ProcessLayer loads and processes a single layer according to its definition
func ProcessLayer(def LayerDefinition, mediaFiles MediaFiles, outputWidth, outputHeight int, resourcesDir string) (*Layer, error) {
	// Load the source image
	img, err := loadLayerImage(def.Resource, mediaFiles, resourcesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load layer resource '%s': %v", def.Resource, err)
	}

	// Calculate layer dimensions
	layerWidth, layerHeight, err := calculateLayerSize(def, img, outputWidth, outputHeight)
	if err != nil {
		return nil, err
	}

	// Resize image if needed
	if layerWidth != img.Bounds().Dx() || layerHeight != img.Bounds().Dy() {
		img = imaging.Resize(img, layerWidth, layerHeight, imaging.Lanczos)
	}

	// Apply effects
	for _, effectDef := range def.Effects {
		effect, err := GetEffect(effectDef.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to get effect '%s': %v", effectDef.Type, err)
		}

		if err := effect.Validate(effectDef.Params); err != nil {
			return nil, fmt.Errorf("effect '%s' validation failed: %v", effectDef.Type, err)
		}

		img, err = effect.Apply(img, effectDef.Params)
		if err != nil {
			return nil, fmt.Errorf("failed to apply effect '%s': %v", effectDef.Type, err)
		}
	}

	// Calculate position
	x, y, err := calculateLayerPosition(def, img.Bounds().Dx(), img.Bounds().Dy(), outputWidth, outputHeight)
	if err != nil {
		return nil, err
	}

	return &Layer{
		Image:  img,
		X:      x,
		Y:      y,
		Width:  img.Bounds().Dx(),
		Height: img.Bounds().Dy(),
	}, nil
}

// loadLayerImage loads an image based on the resource type
func loadLayerImage(resource string, mediaFiles MediaFiles, resourcesDir string) (image.Image, error) {
	var filePath string

	// Check if it's a custom resource
	if strings.HasPrefix(resource, "custom:") {
		customPath := strings.TrimPrefix(resource, "custom:")
		filePath = resourcesDir + "/" + customPath
	} else {
		// Look up media file path
		path, ok := mediaFiles[resource]
		if !ok {
			return nil, fmt.Errorf("media file not found for resource type '%s'", resource)
		}
		filePath = path
	}

	// Load image
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	return img, nil
}

// calculateLayerSize determines the final size of a layer
func calculateLayerSize(def LayerDefinition, img image.Image, outputWidth, outputHeight int) (int, int, error) {
	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	// Parse width
	width := imgWidth
	if def.Width != "" {
		w, err := parseSize(def.Width, outputWidth)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid width: %v", err)
		}
		if w > 0 {
			width = w
		}
	}

	// Parse height
	height := imgHeight
	if def.Height != "" {
		h, err := parseSize(def.Height, outputHeight)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid height: %v", err)
		}
		if h > 0 {
			height = h
		}
	}

	// Handle "auto" for maintaining aspect ratio
	if def.Width == "auto" && def.Height != "" && def.Height != "auto" {
		// Width is auto, calculate from height
		aspectRatio := float64(imgWidth) / float64(imgHeight)
		width = int(float64(height) * aspectRatio)
	} else if def.Height == "auto" && def.Width != "" && def.Width != "auto" {
		// Height is auto, calculate from width
		aspectRatio := float64(imgHeight) / float64(imgWidth)
		height = int(float64(width) * aspectRatio)
	} else if def.Width == "auto" && def.Height == "auto" {
		// Both auto, use original size
		width = imgWidth
		height = imgHeight
	}

	return width, height, nil
}

// calculateLayerPosition determines the position of a layer
func calculateLayerPosition(def LayerDefinition, layerWidth, layerHeight, outputWidth, outputHeight int) (int, int, error) {
	// Parse X position (as offset, could be negative)
	xOffset := 0
	if def.X != "" {
		offset, err := parseOffset(def.X, outputWidth)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid x position: %v", err)
		}
		xOffset = offset
	}

	// Parse Y position (as offset, could be negative)
	yOffset := 0
	if def.Y != "" {
		offset, err := parseOffset(def.Y, outputHeight)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid y position: %v", err)
		}
		yOffset = offset
	}

	// Apply horizontal alignment
	var x int
	switch strings.ToLower(def.Align) {
	case "center", "centre":
		x = (outputWidth-layerWidth)/2 + xOffset
	case "right":
		// Negative offset means "from right edge"
		x = outputWidth - layerWidth + xOffset
	case "left", "":
		// Positive offset from left edge
		x = xOffset
	default:
		return 0, 0, fmt.Errorf("invalid align value: %s", def.Align)
	}

	// Apply vertical alignment
	var y int
	switch strings.ToLower(def.Valign) {
	case "middle", "center", "centre":
		y = (outputHeight-layerHeight)/2 + yOffset
	case "bottom":
		// Negative offset means "from bottom edge"
		y = outputHeight - layerHeight + yOffset
	case "top", "":
		// Positive offset from top edge
		y = yOffset
	default:
		return 0, 0, fmt.Errorf("invalid valign value: %s", def.Valign)
	}

	return x, y, nil
}

// parseSize parses a size string (pixels, percentage, or "auto")
func parseSize(size string, containerSize int) (int, error) {
	size = strings.TrimSpace(size)

	if size == "auto" {
		return -1, nil // Special value indicating auto
	}

	// Check for percentage
	if strings.HasSuffix(size, "%") {
		percentStr := strings.TrimSuffix(size, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid percentage: %s", size)
		}
		return int(float64(containerSize) * percent / 100.0), nil
	}

	// Parse as pixels
	pixels, err := strconv.Atoi(size)
	if err != nil {
		return 0, fmt.Errorf("invalid size value: %s", size)
	}

	return pixels, nil
}

// parseOffset parses an offset string (pixels or percentage, can be negative)
// This is used for x/y positions where alignment context determines the meaning
func parseOffset(offset string, containerSize int) (int, error) {
	offset = strings.TrimSpace(offset)

	// Check for percentage
	if strings.HasSuffix(offset, "%") {
		percentStr := strings.TrimSuffix(offset, "%")
		percent, err := strconv.ParseFloat(percentStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid percentage: %s", offset)
		}
		return int(float64(containerSize) * percent / 100.0), nil
	}

	// Parse as integer (supports negative values)
	value, err := strconv.Atoi(offset)
	if err != nil {
		return 0, fmt.Errorf("invalid offset value: %s", offset)
	}

	return value, nil
}
