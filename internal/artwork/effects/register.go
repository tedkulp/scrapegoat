package effects

import (
	"github.com/tedkulp/scrapegoat/internal/artwork"
)

func init() {
	// Register all Phase 1 effects
	artwork.RegisterEffect(&RoundedEffect{})
	artwork.RegisterEffect(&ShadowEffect{})
	artwork.RegisterEffect(&StrokeEffect{})
	artwork.RegisterEffect(&OpacityEffect{})
	artwork.RegisterEffect(&BlurEffect{})
}
