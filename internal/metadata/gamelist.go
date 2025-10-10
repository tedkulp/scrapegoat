package metadata

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tedkulp/scrapegoat/internal/scraper"
)

// Generator handles gamelist.xml generation
type Generator struct {
	romDir             string
	mediaDir           string
	gameList           *GameList
	preferredRegions   []string
	preferredLanguages []string
}

// NewGenerator creates a new gamelist.xml generator
func NewGenerator(romDir, mediaDir string) *Generator {
	return &Generator{
		romDir:             romDir,
		mediaDir:           mediaDir,
		gameList:           NewGameList(),
		preferredRegions:   []string{"us", "wor", "eu", "jp"},
		preferredLanguages: []string{"en", "us", "fr", "de", "es", "it", "pt"},
	}
}

// SetPreferredRegions sets the preferred region order for metadata
func (g *Generator) SetPreferredRegions(regions []string) {
	g.preferredRegions = regions
}

// SetPreferredLanguages sets the preferred language order for text
func (g *Generator) SetPreferredLanguages(languages []string) {
	g.preferredLanguages = languages
}

// AddGameFromScraper converts ScreenScraper game data to gamelist.xml entry
func (g *Generator) AddGameFromScraper(romPath string, game *scraper.Game, mediaFiles map[string]string) error {
	// ROM path should be relative to gamelist.xml location (which is in romDir)
	// EmulationStation expects paths like "./GameName.zip"
	romFilename := filepath.Base(romPath)
	relRomPath := "./" + romFilename

	// Get only the first genre and trim whitespace
	genreStr := strings.TrimSpace(game.GetFirstGenre())

	gameEntry := Game{
		Path:        relRomPath,
		Name:        strings.TrimSpace(game.GetPreferredName(g.preferredRegions)),
		Desc:        strings.TrimSpace(game.GetPreferredSynopsis(g.preferredLanguages)),
		Developer:   strings.TrimSpace(game.Developer.Name),
		Publisher:   strings.TrimSpace(game.Publisher.Name),
		Genre:       genreStr,
		Players:     strings.TrimSpace(game.Players.Text),
		ReleaseDate: formatReleaseDate(game.GetReleaseDate(g.preferredRegions)),
	}

	// Parse rating (convert to 0-1 scale if needed)
	if len(game.Ratings) > 0 {
		if rating, err := parseRating(game.Ratings[0].Value); err == nil {
			gameEntry.Rating = rating
		}
	}

	// Note: Media tags are not included in gamelist.xml
	// EmulationStation automatically looks for media files by convention:
	// - Images in subdirectories like wheel/, screenshots/, etc.
	// - Files should match the ROM name (without extension)

	g.gameList.AddGame(gameEntry)
	return nil
}

// AddGame adds a game entry directly
func (g *Generator) AddGame(game Game) {
	g.gameList.AddGame(game)
}

// WriteToFile writes the gamelist.xml to disk
func (g *Generator) WriteToFile(filepath string) error {
	// Create directory if it doesn't exist
	dir := filepath[:len(filepath)-len(filepath[strings.LastIndex(filepath, "/"):])]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file for writing
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write XML header
	file.WriteString(xml.Header)

	// Create encoder with indentation
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")

	// Encode the game list
	if err := encoder.Encode(g.gameList); err != nil {
		return fmt.Errorf("failed to encode XML: %w", err)
	}

	// Add final newline
	file.WriteString("\n")

	return nil
}

// formatReleaseDate converts various date formats to YYYYMMDDTHHMISS format
func formatReleaseDate(date string) string {
	if date == "" {
		return ""
	}

	// ScreenScraper returns dates in YYYY-MM-DD format
	// EmulationStation uses YYYYMMDDTHHMISS format
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")
	date = strings.TrimSpace(date)

	// Pad the date to ensure it's YYYYMMDD format
	// If we have partial dates like "1990" -> "19900101"
	// If we have "199001" -> "19900101"
	// If we have "19900215" -> "19900215"
	switch len(date) {
	case 0:
		return ""
	case 4: // YYYY
		date = date + "0101" // January 1st
	case 6: // YYYYMM
		date = date + "01" // First day of month
	case 8: // YYYYMMDD (already complete)
		// Keep as-is
	default:
		// If longer than 8, truncate to 8
		if len(date) > 8 {
			date = date[:8]
		} else if len(date) < 4 {
			// Too short to be valid, return empty
			return ""
		}
	}

	// Ensure we have exactly 8 characters before adding time
	if len(date) == 8 {
		return date + "T000000"
	}

	return ""
}

// parseRating converts rating string to float64 (0-1 scale)
func parseRating(ratingStr string) (float64, error) {
	rating, err := strconv.ParseFloat(ratingStr, 64)
	if err != nil {
		return 0, err
	}

	// If rating is on 0-5 or 0-10 scale, convert to 0-1
	if rating > 1 {
		if rating <= 5 {
			rating = rating / 5.0
		} else if rating <= 10 {
			rating = rating / 10.0
		} else if rating <= 20 {
			rating = rating / 20.0
		}
	}

	return rating, nil
}
