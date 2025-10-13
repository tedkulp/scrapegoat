# Artwork Generation

Scrapegoat includes a powerful artwork compositing system that allows you to create custom composite images (miximages) from downloaded media files. This feature is inspired by Skyscraper's artwork.xml functionality.

## Overview

The artwork system:
- Combines multiple media sources (screenshots, box art, logos, etc.) into single composite images
- Supports layering with precise positioning and alignment
- Provides visual effects (shadows, rounded corners, borders, blur, opacity)
- Uses intelligent caching to avoid regenerating identical artwork
- Gracefully handles missing media files

## Quick Start

### Enable Artwork Generation

Add the `artwork` section to your `config.yaml`:

```yaml
artwork:
  enabled: true
  output_dir: "miximages"

  outputs:
    - type: screenshot
      width: 640
      height: 480
      format: png
      background: "transparent"

      layers:
        - resource: screenshots
          width: 520
          height: 390
          x: 20
          align: center
          valign: middle
```

### Command-Line Control

You can override the config file setting with the `--artwork` flag:

```bash
# Enable artwork (overrides config)
./scrapegoat scrape --platform nes --rom-dir ./roms --artwork true

# Disable artwork (overrides config)
./scrapegoat scrape --platform nes --rom-dir ./roms --artwork false

# Use config setting (default)
./scrapegoat scrape --platform nes --rom-dir ./roms
```

### Verbose Output

Use `--verbose` to see detailed artwork generation logs:

```bash
./scrapegoat scrape --platform nes --rom-dir ./roms --verbose
```

This will show:
- Which layers are being processed
- Applied effects for each layer
- Cache hits/misses
- Layers skipped due to missing media
- Final output paths

## Configuration Reference

### Top-Level Settings

```yaml
artwork:
  enabled: true              # Enable/disable artwork generation
  output_dir: "miximages"    # Directory for generated artwork (relative to media root)
  resources_dir: "~/.scrapegoat/resources"  # Directory for custom resources
```

### Output Definitions

Each output defines a single composite image type:

```yaml
outputs:
  - type: screenshot         # Identifier for this output (used in filename)
    width: 640              # Canvas width in pixels
    height: 480             # Canvas height in pixels
    format: png             # Output format: png or jpg
    background: "transparent"  # Background color or "transparent"

    layers:
      # Layer definitions (see below)
```

### Layer Definitions

Layers are composited bottom-to-top (first layer = base):

```yaml
layers:
  - resource: screenshots    # Media type to use (see Available Resources)
    width: 520              # Layer width (pixels or "auto")
    height: 390             # Layer height (pixels or "auto")
    x: 20                   # X position (pixels, percentage, or offset)
    y: 0                    # Y position (pixels, percentage, or offset)
    align: center           # Horizontal alignment: left, center, right
    valign: middle          # Vertical alignment: top, middle, bottom

    effects:
      # Effect definitions (see below)
```

#### Available Resources

Resources are media types downloaded from ScreenScraper:

- `screenshots` - In-game screenshots
- `titlescreens` - Title screen images
- `box2dfront` - 2D box/cover art
- `3dboxes` - 3D box renders
- `backcovers` - Back cover art
- `wheel` - Logo/wheel images
- `physicalmedia` - Physical media (cartridge/disc)
- `manuals` - Manual scans
- `videos` - Gameplay videos

#### Positioning and Alignment

**Width/Height:**
- Specific pixels: `width: 520`
- Auto-calculate: `width: auto` (maintains aspect ratio)
- Percentage: `width: "50%"` (of canvas size)

**Position (X/Y):**
- Absolute pixels: `x: 20` (20 pixels from alignment edge)
- Percentage: `x: "10%"` (10% of canvas width)
- Negative offsets: `x: -10` (10 pixels from opposite edge)

**Alignment:**
- `align: left` - Align to left edge (x is offset from left)
- `align: center` - Center horizontally (x is offset from center)
- `align: right` - Align to right edge (x is offset from right, use negative values)
- `valign: top` - Align to top edge (y is offset from top)
- `valign: middle` - Center vertically (y is offset from center)
- `valign: bottom` - Align to bottom edge (y is offset from bottom, use negative values)

**Examples:**

```yaml
# Position 20px from left edge
x: 20
align: left

# Position 10px from right edge (note negative offset)
x: -10
align: right

# Center horizontally, offset 5px right
x: 5
align: center

# Position 10px from bottom edge
y: -10
valign: bottom
```

### Effects

Effects are applied to layers in the order specified:

#### Shadow

Adds a drop shadow with blur:

```yaml
effects:
  - type: shadow
    distance: 5        # Shadow offset distance (pixels)
    softness: 5        # Blur radius (pixels)
    opacity: 70        # Shadow opacity (0-100)
    angle: 135         # Shadow angle (degrees: 0=right, 90=down, 180=left, 270=up)
    color: "#000000"   # Shadow color (hex)
```

#### Rounded Corners

Applies rounded corners:

```yaml
effects:
  - type: rounded
    radius: 10         # Corner radius (pixels)
```

#### Stroke/Border

Adds an outline border:

```yaml
effects:
  - type: stroke
    width: 5           # Border width (pixels)
    color: "#FFFFFF"   # Border color (hex)
    opacity: 100       # Border opacity (0-100)
```

#### Opacity

Adjusts layer transparency:

```yaml
effects:
  - type: opacity
    value: 80          # Opacity value (0-100, 100=opaque)
```

#### Blur

Applies Gaussian blur:

```yaml
effects:
  - type: blur
    radius: 5          # Blur radius (pixels)
```

## Complete Example

This example creates a composite image combining a screenshot with rounded corners and border, a 3D box with shadow in the corner, and a wheel logo in the top-right:

```yaml
artwork:
  enabled: true
  output_dir: "miximages"
  resources_dir: "~/.scrapegoat/resources"

  outputs:
    - type: screenshot
      width: 640
      height: 480
      format: png
      background: "transparent"

      layers:
        # Layer 1: Main screenshot with rounded corners and white border
        - resource: screenshots
          x: 20
          y: 0
          width: 520
          height: 390
          align: center
          valign: middle
          effects:
            - type: rounded
              radius: 10
            - type: stroke
              width: 5
              color: "#FFFFFF"
              opacity: 100

        # Layer 2: 3D box in bottom-left with shadow
        - resource: 3dboxes
          height: 250
          width: auto
          x: 0
          y: -10
          align: left
          valign: bottom
          effects:
            - type: shadow
              distance: 5
              softness: 5
              opacity: 70
              angle: 45
              color: "#000000"

        # Layer 3: Wheel logo in top-right with shadow
        - resource: wheel
          width: 250
          height: auto
          x: -10
          y: 0
          align: right
          valign: top
          effects:
            - type: shadow
              distance: 5
              softness: 5
              opacity: 70
              angle: 45
              color: "#000000"

        # Layer 4: Physical media (cartridge) if available
        - resource: physicalmedia
          width: 130
          height: auto
          x: 150
          y: 0
          align: left
          valign: bottom
          effects:
            - type: shadow
              distance: 5
              softness: 5
              opacity: 70
              angle: 45
              color: "#000000"
```

## How It Works

### Layer Processing

1. **First Layer Required**: The first layer's media file must exist. If it's missing, artwork generation is skipped for that game.

2. **Optional Subsequent Layers**: Layers 2+ are optional. If their media files are missing, they are silently skipped, and remaining layers are processed normally.

3. **Effect Pipeline**: Effects are applied to each layer in order before compositing onto the canvas.

4. **Bottom-to-Top Compositing**: Layers are drawn in order, with later layers appearing on top of earlier ones.

### Caching

The artwork system uses intelligent caching:

- **Cache Key**: Generated from game name, output settings, media files, and modification times
- **Cache Location**: `~/.scrapegoat/artwork-cache/` (organized into subdirectories)
- **Cache Validation**: If any media file changes, cache is invalidated automatically
- **Performance**: Cached artwork is copied directly to output without regeneration

### Output Files

Generated artwork files are saved to:
```
{media-root-dir}/{output_dir}/{game-name}.{format}
```

Example:
```
/roms/nes/miximages/Super Mario Bros.png
```

The files are also included in `gamelist.xml` with proper relative paths for EmulationStation compatibility.

## Troubleshooting

### No artwork generated

Check that:
- `artwork.enabled: true` in `config.yaml`
- At least the first layer's media exists for each game
- The `--dry-run` flag is not set
- Output directory is writable

### Missing layers

This is normal behavior:
- First layer missing = skip artwork for that game
- Other layers missing = continue with remaining layers
- Use `--verbose` to see which layers are skipped

### Effects not visible

Check:
- Effect parameters are within valid ranges
- Layer is large enough for effect to be visible (especially shadows)
- Background allows transparency (use PNG format)
- Opacity values are not too low

### Poor performance

The artwork system is optimized with caching. First run will be slow, subsequent runs will be fast. To improve performance:
- Ensure cache directory (`~/.scrapegoat`) is on a fast disk
- Reduce image sizes if possible
- Use fewer effects
- Consider disabling artwork with `--artwork false` for quick metadata-only runs

## Tips and Best Practices

1. **Start Simple**: Begin with a basic 2-layer composition, then add complexity
2. **Test Small**: Use `--verbose` and test with a few ROMs first
3. **Mind Aspect Ratios**: Use `width: auto` or `height: auto` to maintain proper proportions
4. **Layer Order Matters**: Background elements should be first, foreground elements last
5. **Negative Offsets**: For right/bottom alignment, use negative x/y values
6. **EmulationStation Compatibility**: Use PNG format for transparency support
7. **Performance**: Cache makes reruns fast - don't worry about generation time on first run

## Integration with EmulationStation

Generated artwork is automatically:
- Saved to the configured output directory
- Added to `gamelist.xml` with proper relative paths
- Organized alongside other media files
- Named to match ROM files (without extension)

EmulationStation will automatically display the generated artwork when browsing your game collection.
