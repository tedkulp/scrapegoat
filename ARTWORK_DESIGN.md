# Artwork Compositing System - Design Document

## Overview

This document outlines the design for an artwork compositing system for Scrapegoat, inspired by Skyscraper's artwork.xml system. The goal is to provide flexible, declarative image composition and transformation capabilities using YAML configuration.

## Goals

1. **Flexibility**: Support complex image compositions with multiple layers
2. **Declarative**: Configuration-driven, no code changes needed for new artwork styles
3. **EmulationStation Compatible**: Generate artwork that works with ES themes
4. **Extensible**: Easy to add new effects and layer types
5. **Performant**: Process images efficiently, cache when possible

## Non-Goals (for v1)

1. Real-time preview UI
2. Video processing/compositing
3. Dynamic artwork based on game metadata variables (e.g., {game_name})
4. Conditional logic in configs (if/else statements)

## Architecture

### Package Structure

```
internal/artwork/
  ├── compositor.go      # Main compositor engine
  ├── layer.go          # Layer representation and processing
  ├── effects.go        # Effect definitions and registry
  ├── effects/          # Individual effect implementations
  │   ├── rounded.go
  │   ├── shadow.go
  │   ├── stroke.go
  │   └── ...
  └── types.go          # Shared types and interfaces
```

### Dependencies

**Recommended: github.com/disintegration/imaging**
- Pure Go (no CGO, easier to build/deploy)
- Good performance for basic operations
- Simpler API than ImageMagick
- Supports common image formats (PNG, JPEG, GIF)

**Alternative: github.com/gographics/imagick**
- Full ImageMagick feature parity
- Requires CGO and ImageMagick installation
- More complex operations (3D transforms, advanced filters)
- Consider if users request advanced effects

**Decision: Start with `imaging`, migrate to `imagick` if needed**

## YAML Configuration Structure

### Location

Artwork configs can be defined in two places:
1. **Global**: `config.yaml` under `artwork:` section
2. **Per-platform**: Platform-specific overrides (future enhancement)

### Schema

```yaml
artwork:
  # Enable/disable artwork generation
  enabled: true

  # Output directory for generated artwork (relative to media-root-dir)
  output_dir: "artwork"

  # Resource directories for custom images (frames, masks, etc.)
  resources_dir: "~/.scrapegoat/resources"

  # Output definitions - what final images to generate
  outputs:
    # Each output type generates one image per game
    - type: screenshot           # Output filename: {game}-screenshot.png
      width: 640                 # Optional: target width (default: auto)
      height: 480                # Optional: target height (default: auto)
      format: png                # Output format: png, jpg, webp
      background: "#000000"      # Background color (hex or transparent)

      # Layers are rendered bottom-to-top (first = bottom)
      layers:
        # Layer 1: Base screenshot
        - resource: screenshot   # Source media type
          x: 20                  # X position (pixels, or %)
          y: 10                  # Y position
          width: 520             # Width (pixels, % of output, or "auto")
          height: 390            # Height
          align: center          # Horizontal align: left, center, right
          valign: middle         # Vertical align: top, middle, bottom

          # Effects applied in order (top to bottom)
          effects:
            - type: rounded
              radius: 10         # Corner radius in pixels

            - type: stroke
              width: 5           # Border width in pixels
              color: "#FFFFFF"   # Border color
              opacity: 100       # 0-100

        # Layer 2: Game cover (on top)
        - resource: cover
          height: 250
          x: 0
          y: -10               # Negative = offset from edge
          valign: bottom       # Anchor to bottom

          effects:
            - type: shadow
              distance: 5      # Shadow offset in pixels
              softness: 5      # Blur radius
              opacity: 70      # 0-100
              angle: 135       # Shadow direction in degrees

    # Additional output types
    - type: marquee
      width: 640
      height: 160
      layers:
        - resource: wheel
          align: center
          valign: middle
          effects:
            - type: glow
              radius: 10
              color: "#00FF00"
              opacity: 50
```

### Resource Types

Media types that can be used as layer sources:
- `screenshot` - In-game screenshot
- `cover` / `box2dfront` - Box art / cover
- `wheel` - Logo/wheel art
- `titlescreen` - Title screen
- `marquee` - Marquee/banner art
- `video-frame` - First frame of video (future)
- `custom:{path}` - Custom image from resources_dir

### Position and Alignment

**Absolute Positioning (pixels):**
```yaml
x: 100        # 100px from left
y: 50         # 50px from top
```

**Relative Positioning (percentage):**
```yaml
x: 50%        # Center horizontally
y: 25%        # Quarter from top
```

**Negative Offsets (from opposite edge):**
```yaml
x: -20        # 20px from right edge
y: -50        # 50px from bottom edge
```

**Alignment:**
```yaml
align: left    # Horizontal: left, center, right
valign: top    # Vertical: top, middle, bottom
```

**Size:**
```yaml
width: 640         # Exact pixels
height: 50%        # 50% of output height
width: auto        # Maintain aspect ratio based on height
```

## Effects System

### Effect Interface

```go
type Effect interface {
    // Apply the effect to an image
    Apply(img image.Image, params map[string]interface{}) (image.Image, error)

    // Validate effect parameters
    Validate(params map[string]interface{}) error

    // Get effect name
    Name() string
}
```

### Effect Registry

Effects are registered at init time and looked up by name:

```go
var effectRegistry = make(map[string]Effect)

func RegisterEffect(effect Effect) {
    effectRegistry[effect.Name()] = effect
}

func GetEffect(name string) (Effect, error) {
    effect, ok := effectRegistry[name]
    if !ok {
        return nil, fmt.Errorf("unknown effect: %s", name)
    }
    return effect, nil
}
```

### Phase 1 Effects (MVP)

**Essential effects for basic compositions:**

#### 1. Rounded Corners
```yaml
- type: rounded
  radius: 10         # Corner radius in pixels
```

**Implementation:** Create corner masks and apply transparency

#### 2. Shadow
```yaml
- type: shadow
  distance: 5        # Offset distance
  softness: 5        # Blur radius
  opacity: 70        # 0-100
  angle: 135         # Direction (0=right, 90=down, etc)
  color: "#000000"   # Shadow color
```

**Implementation:** Create shadow layer, blur, offset, composite

#### 3. Stroke (Border)
```yaml
- type: stroke
  width: 5           # Border width in pixels
  color: "#FFFFFF"   # Border color
  opacity: 100       # 0-100
```

**Implementation:** Draw border around image edge

#### 4. Opacity
```yaml
- type: opacity
  value: 50          # 0-100
```

**Implementation:** Adjust alpha channel

#### 5. Blur
```yaml
- type: blur
  radius: 5          # Blur radius in pixels
```

**Implementation:** Gaussian blur filter

### Phase 2 Effects (Future)

More advanced effects to add later:

#### 6. Colorize
```yaml
- type: colorize
  hue: 180           # Hue rotation (0-360)
  saturation: 50     # Saturation adjustment (-100 to 100)
```

#### 7. Brightness/Contrast
```yaml
- type: brightness
  value: 10          # -100 to 100

- type: contrast
  value: 20          # -100 to 100
```

#### 8. Rotate
```yaml
- type: rotate
  degrees: 45        # Rotation angle
  axis: z            # Rotation axis: x, y, z (3D)
```

#### 9. Frame
```yaml
- type: frame
  image: "custom_frame.png"  # Frame image from resources_dir
  mode: stretch              # How to apply: stretch, tile, corner
```

#### 10. Mask
```yaml
- type: mask
  image: "circle_mask.png"   # Mask image (white=opaque, black=transparent)
```

#### 11. Glow
```yaml
- type: glow
  radius: 10         # Glow size
  color: "#00FF00"   # Glow color
  opacity: 50        # 0-100
```

## Integration with Scraper

### Workflow

1. **Scraping Phase**: Download all media files as normal
2. **Artwork Phase**: If `artwork.enabled: true`, generate composed images
3. **Output Phase**: Write gamelist.xml with artwork references

### Command Line Flag

Add optional flag to control artwork generation:
```bash
scrapegoat scrape --platform nes --rom-dir roms --artwork=true
```

Also respect config:
```yaml
artwork:
  enabled: true  # Can be overridden by --artwork flag
```

### Caching Strategy

Generated artwork should be cached to avoid regeneration:

**Cache Location:** `~/.scrapegoat/artwork-cache/{platform}/{game}-{output-type}-{hash}.png`

**Hash Calculation:** MD5 of:
1. Source media file paths and their modification times
2. Artwork config YAML for this output type
3. Scrapegoat version

**Cache Invalidation:**
- Source media changed
- Config changed
- Scrapegoat updated (version change)

### Error Handling

**Missing Source Media:**
- Log warning: "Skipping artwork for {game}: missing {resource}"
- Continue processing other games
- Don't fail entire scrape

**Effect Errors:**
- Log error with effect name and parameters
- Skip this output type for this game
- Continue with other output types

**Resource Errors:**
- Validate resources_dir exists at startup
- Warn if custom resources referenced but not found
- Provide clear error messages

## Configuration Examples

### Example 1: Simple Screenshot Enhancement

```yaml
artwork:
  enabled: true
  outputs:
    - type: screenshot
      width: 640
      height: 480
      layers:
        - resource: screenshot
          align: center
          valign: middle
          effects:
            - type: rounded
              radius: 15
            - type: shadow
              distance: 8
              softness: 10
              opacity: 50
```

**Output:** Rounded screenshot with drop shadow

### Example 2: Cover + Screenshot Composite

```yaml
artwork:
  enabled: true
  outputs:
    - type: composite
      width: 800
      height: 600
      background: "#1a1a1a"
      layers:
        # Background screenshot (blurred)
        - resource: screenshot
          width: 100%
          height: 100%
          effects:
            - type: blur
              radius: 20
            - type: opacity
              value: 30

        # Main screenshot (centered)
        - resource: screenshot
          width: 60%
          align: center
          valign: middle
          effects:
            - type: rounded
              radius: 10
            - type: stroke
              width: 3
              color: "#FFFFFF"

        # Cover box (bottom-right)
        - resource: cover
          height: 40%
          x: -20
          y: -20
          align: right
          valign: bottom
          effects:
            - type: shadow
              distance: 10
              softness: 15
              opacity: 80
```

**Output:** Artistic composite with blurred background, centered screenshot, and cover in corner

### Example 3: Wheel Logo with Glow

```yaml
artwork:
  enabled: true
  outputs:
    - type: logo
      width: 400
      height: 200
      background: transparent
      layers:
        - resource: wheel
          align: center
          valign: middle
          effects:
            - type: glow
              radius: 15
              color: "#00FF00"
              opacity: 60
            - type: shadow
              distance: 5
              softness: 8
              opacity: 40
```

**Output:** Wheel logo with green glow effect

## Implementation Phases

### Phase 1: Foundation (MVP)
- [ ] Image loading and basic composition
- [ ] Layer positioning (x, y, align, valign)
- [ ] Layer scaling (width, height)
- [ ] Basic effects: rounded, shadow, stroke, opacity
- [ ] Single output type support
- [ ] Integration with scraper workflow

**Estimated effort:** 2-3 days

### Phase 2: Enhanced Effects
- [ ] Additional effects: blur, colorize, brightness, contrast
- [ ] Multiple output types
- [ ] Custom resources support
- [ ] Effect chaining optimization

**Estimated effort:** 1-2 days

### Phase 3: Advanced Features
- [ ] 3D transforms (rotate with axis)
- [ ] Frame and mask effects
- [ ] Percentage-based positioning
- [ ] Per-platform config overrides
- [ ] Artwork preview/validation tool

**Estimated effort:** 2-3 days

## Testing Strategy

### Unit Tests
- Individual effect processors
- Position calculation (absolute, relative, negative)
- Alignment logic
- Config parsing and validation

### Integration Tests
- End-to-end artwork generation
- Multiple layer composition
- Effect chaining
- Error handling

### Manual Testing
- Various image sizes and aspect ratios
- Missing source media handling
- Performance with large images
- Memory usage

## Open Questions

1. **Output Format:** Should we support multiple output formats (PNG, JPG, WebP) or just PNG?
   - **Recommendation:** PNG for quality, add JPG/WebP later if needed

2. **Performance:** Should we process artwork in parallel (goroutines)?
   - **Recommendation:** Yes, process games concurrently with worker pool

3. **Config Validation:** When should we validate artwork config?
   - **Recommendation:** At startup, fail fast with clear errors

4. **EmulationStation Integration:** How should ES discover the artwork?
   - **Recommendation:** Write to `artwork/` subdir, update gamelist.xml with paths (future enhancement)

5. **Custom Resources:** Should we bundle default frames/masks?
   - **Recommendation:** No, keep binary small. Provide examples in docs.

6. **Effect Ordering:** Does effect order matter?
   - **Recommendation:** Yes, apply effects in YAML order (allows rounded-then-shadow)

## Future Enhancements

- **Variables:** Support `{game}`, `{platform}`, `{year}` in config
- **Conditions:** Conditional layers based on available media
- **Templates:** Reusable layer/effect templates
- **Themes:** Predefined artwork configs (retro, modern, minimal)
- **CLI Tool:** `scrapegoat artwork generate` for batch processing
- **Preview Mode:** Generate samples without full scrape

## Migration from Skyscraper

For users migrating from Skyscraper, provide conversion tool:

```bash
scrapegoat convert-artwork ~/.skyscraper/artwork.xml
```

Outputs equivalent YAML config with notes on unsupported features.

## Documentation Requirements

1. **User Guide:**
   - Configuration overview
   - Layer positioning explained with diagrams
   - Effect reference (all parameters)
   - Example configurations
   - Troubleshooting

2. **Developer Guide:**
   - Adding new effects
   - Effect interface documentation
   - Testing guidelines
   - Performance considerations

## Summary

This design provides a solid foundation for artwork composition in Scrapegoat while maintaining simplicity and extensibility. The YAML-based configuration is more readable than XML and integrates naturally with the existing config system.

Key decisions:
- **Pure Go library** (`imaging`) for easier deployment
- **Phase 1 focuses on MVP** (basic composition + essential effects)
- **Clear separation** between compositor, layers, and effects
- **Flexible configuration** with sensible defaults
- **Extensible architecture** for future enhancements

Next steps after review:
1. Finalize YAML schema
2. Implement foundation (compositor + layer positioning)
3. Add Phase 1 effects
4. Integrate with scraper
5. Write documentation and examples
