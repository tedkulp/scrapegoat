package platform

// aliases are slugs Scrapegoat adds where none of a platform's own names
// produce the one users expect. Each names the ScreenScraper platform ID it
// picks; an alias beats any name slug, and only a config.yaml platform beats
// an alias. A platform has at most one alias, which list-platforms shows.
var aliases = []struct {
	slug string
	id   int
}{
	{"virtualboy", 11},   // Nintendo virtualboy
	{"ngp", 25},          // SNK Neo-Geo Pocket
	{"atari2600", 26},    // Atari2600
	{"tg16", 31},         // TurboGrafx-16
	{"atari5200", 40},    // Atari 5200
	{"gameandwatch", 52}, // Nintendo Game & Watch
	{"n64dd", 122},       // Nintendo 64 DD
	// MS-DOS (135) also lists PrBoom among its names and has the lower ID;
	// without this alias the PrBoom platform would have no slug at all.
	{"prboom", 290}, // PrBoom
}

func aliasFor(id int) (string, bool) {
	for _, a := range aliases {
		if a.id == id {
			return a.slug, true
		}
	}
	return "", false
}
