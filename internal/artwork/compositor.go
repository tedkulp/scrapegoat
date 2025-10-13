package artwork

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// Compositor handles artwork generation
type Compositor struct {
	config       Config
	cacheDir     string
	resourcesDir string
	verbose      bool
}

// NewCompositor creates a new compositor with the given configuration
func NewCompositor(config Config, cacheDir string) *Compositor {
	return &Compositor{
		config:       config,
		cacheDir:     cacheDir,
		resourcesDir: expandHomeDir(config.ResourcesDir),
		verbose:      false,
	}
}

// SetVerbose enables verbose logging
func (c *Compositor) SetVerbose(verbose bool) {
	c.verbose = verbose
}

func (c *Compositor) logVerbose(format string, args ...interface{}) {
	if c.verbose {
		fmt.Printf("    [Artwork] "+format+"\n", args...)
	}
}

// GenerateArtwork generates artwork for a game
func (c *Compositor) GenerateArtwork(gameName string, mediaFiles MediaFiles, outputDir string) (map[string]string, error) {
	generatedFiles := make(map[string]string)

	c.logVerbose("Generating %d output(s) for %s", len(c.config.Outputs), gameName)

	for _, output := range c.config.Outputs {
		c.logVerbose("Processing output '%s' (%dx%d)", output.Type, output.Width, output.Height)
		outputPath, err := c.generateOutput(gameName, output, mediaFiles, outputDir)
		if err != nil {
			return nil, fmt.Errorf("failed to generate '%s' artwork: %v", output.Type, err)
		}
		c.logVerbose("Generated: %s", filepath.Base(outputPath))
		generatedFiles[output.Type] = outputPath
	}

	return generatedFiles, nil
}

// generateOutput generates a single output image
func (c *Compositor) generateOutput(gameName string, def OutputDefinition, mediaFiles MediaFiles, outputDir string) (string, error) {
	// Check if we have at least the first layer's media file
	if len(def.Layers) == 0 {
		return "", fmt.Errorf("no layers defined in output")
	}

	// Validate first layer exists - this is required
	firstLayerResource := def.Layers[0].Resource
	if !strings.HasPrefix(firstLayerResource, "custom:") {
		if _, ok := mediaFiles[firstLayerResource]; !ok {
			return "", fmt.Errorf("first layer media file not found: %s", firstLayerResource)
		}
	}
	// Additional layers are optional - we'll skip them if media is missing

	// Check cache first
	cacheKey := c.calculateCacheKey(gameName, def, mediaFiles)
	cachedPath, found := c.checkCache(cacheKey, def.Format)
	if found {
		c.logVerbose("Cache hit for '%s'", def.Type)
		// Copy from cache to output directory
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.%s", gameName, def.Type, def.Format))
		if err := copyFile(cachedPath, outputPath); err != nil {
			return "", fmt.Errorf("failed to copy from cache: %v", err)
		}
		return outputPath, nil
	}
	c.logVerbose("Cache miss - generating new artwork")

	// Determine output dimensions
	width := def.Width
	height := def.Height
	if width == 0 || height == 0 {
		// Auto-size based on first layer
		if len(def.Layers) > 0 {
			img, err := loadLayerImage(def.Layers[0].Resource, mediaFiles, c.resourcesDir)
			if err != nil {
				return "", fmt.Errorf("failed to load first layer for auto-sizing: %v", err)
			}
			if width == 0 {
				width = img.Bounds().Dx()
			}
			if height == 0 {
				height = img.Bounds().Dy()
			}
		} else {
			return "", fmt.Errorf("output dimensions required when no layers are defined")
		}
	}

	// Create canvas with background
	canvas, err := c.createCanvas(width, height, def.Background)
	if err != nil {
		return "", err
	}

	// Process and composite each layer
	for i, layerDef := range def.Layers {
		c.logVerbose("Layer %d: resource=%s", i+1, layerDef.Resource)

		// Check if media file exists for this layer (skip if missing, except for first layer)
		if !strings.HasPrefix(layerDef.Resource, "custom:") {
			if _, ok := mediaFiles[layerDef.Resource]; !ok {
				if i == 0 {
					// First layer is required
					return "", fmt.Errorf("first layer media file not found: %s", layerDef.Resource)
				}
				// Skip optional layers if media is missing
				c.logVerbose("Skipping layer %d - media not found", i+1)
				continue
			}
		}

		layer, err := ProcessLayer(layerDef, mediaFiles, width, height, c.resourcesDir)
		if err != nil {
			if i == 0 {
				// First layer errors are fatal
				return "", fmt.Errorf("failed to process first layer: %v", err)
			}
			// Skip optional layers that fail to process
			c.logVerbose("Skipping layer %d - processing failed: %v", i+1, err)
			continue
		}

		c.logVerbose("Compositing layer %d at (%d,%d) size %dx%d", i+1, layer.X, layer.Y, layer.Width, layer.Height)
		if len(layerDef.Effects) > 0 {
			effectNames := make([]string, len(layerDef.Effects))
			for j, e := range layerDef.Effects {
				effectNames[j] = e.Type
			}
			c.logVerbose("Applied effects: %s", strings.Join(effectNames, ", "))
		}

		// Composite layer onto canvas
		draw.Draw(canvas, image.Rect(layer.X, layer.Y, layer.X+layer.Width, layer.Y+layer.Height),
			layer.Image, image.Point{}, draw.Over)
	}

	// Save to cache
	cachedPath, err = c.saveToCache(canvas, cacheKey, def.Format)
	if err != nil {
		return "", fmt.Errorf("failed to save to cache: %v", err)
	}

	// Copy from cache to output directory
	// Use just the game name without the output type suffix for EmulationStation compatibility
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s", gameName, def.Format))
	if err := copyFile(cachedPath, outputPath); err != nil {
		return "", fmt.Errorf("failed to copy to output: %v", err)
	}

	return outputPath, nil
}


// createCanvas creates a new canvas with the specified background
func (c *Compositor) createCanvas(width, height int, background string) (*image.NRGBA, error) {
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))

	if background == "" || background == "transparent" {
		// Transparent background (already default for NRGBA)
		return canvas, nil
	}

	// Parse and fill background color
	bgColor, err := ParseColor(background)
	if err != nil {
		return nil, fmt.Errorf("invalid background color: %v", err)
	}

	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	return canvas, nil
}

// calculateCacheKey generates a cache key for an output
func (c *Compositor) calculateCacheKey(gameName string, def OutputDefinition, mediaFiles MediaFiles) string {
	h := md5.New()

	// Include game name
	h.Write([]byte(gameName))

	// Include output definition (simplified - just the type and dimensions)
	h.Write([]byte(fmt.Sprintf("%s-%d-%d-%s", def.Type, def.Width, def.Height, def.Background)))

	// Include media file paths and their modification times
	for resource, path := range mediaFiles {
		h.Write([]byte(resource))
		h.Write([]byte(path))
		if info, err := os.Stat(path); err == nil {
			h.Write([]byte(info.ModTime().String()))
		}
	}

	// Include layer and effect configuration
	for _, layer := range def.Layers {
		h.Write([]byte(fmt.Sprintf("%+v", layer)))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// checkCache checks if a cached version exists
func (c *Compositor) checkCache(cacheKey, format string) (string, bool) {
	// Use first 2 characters of hash for subdirectory (like git does)
	subdir := cacheKey[:2]
	cachePath := filepath.Join(c.cacheDir, "artwork-cache", subdir, fmt.Sprintf("%s.%s", cacheKey, format))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, true
	}
	return "", false
}

// saveToCache saves an image to the cache
func (c *Compositor) saveToCache(img image.Image, cacheKey, format string) (string, error) {
	// Use first 2 characters of hash for subdirectory (like git does)
	subdir := cacheKey[:2]
	cacheDir := filepath.Join(c.cacheDir, "artwork-cache", subdir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s.%s", cacheKey, format))
	file, err := os.Create(cachePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	switch strings.ToLower(format) {
	case "png", "":
		err = png.Encode(file, img)
	case "jpg", "jpeg":
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", err
	}

	return cachePath, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Read source
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Write destination
	return os.WriteFile(dst, data, 0644)
}

// expandHomeDir expands ~ to the user's home directory
func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
