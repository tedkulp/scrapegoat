package config

// platformAliases maps custom slug aliases to their canonical slugs
// Add entries here when the automatic slug generation doesn't produce the desired result
// The key is the desired alias, the value is the canonical slug that was auto-generated
// To find the canonical slug, run: ./scrapegoat list-platforms
//
// Note: Platform slugs are automatically generated without dashes (e.g., "n64dd", "wiiu")
// so you generally only need aliases for platforms with unconventional slugs.
var platformAliases = map[string]string{
	"atari2600":    "2600",
	"atari5200":    "5200",
	"gameandwatch": "gw",
	"ngp":          "gp",
	"n64dd":        "64dd",
	"virtualboy":   "vboy",
	"tg16":         "turbografx16",
}

// applyPlatformAliases adds alias mappings to the platforms cache
// This allows users to use common/expected slug names that might not be
// generated automatically by the slug library
func applyPlatformAliases(platforms map[string]Platform) {
	for alias, canonicalSlug := range platformAliases {
		// Find the platform with the canonical slug
		if platform, ok := platforms[canonicalSlug]; ok {
			// Add the alias pointing to the same platform
			platforms[alias] = platform
		}
	}
}

// GetPreferredSlug returns the alias slug if one exists, otherwise returns the canonical slug
// This is used for display purposes to show users the recommended slug to use
func GetPreferredSlug(canonicalSlug string) string {
	// Check if there's an alias that points to this canonical slug
	for alias, canonical := range platformAliases {
		if canonical == canonicalSlug {
			return alias
		}
	}
	return canonicalSlug
}
