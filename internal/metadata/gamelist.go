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
	// Calculate relative ROM path
	relRomPath, err := filepath.Rel(g.romDir, romPath)
	if err != nil {
		return fmt.Errorf("failed to get relative ROM path: %w", err)
	}

	// Start with "./" for EmulationStation
	if !strings.HasPrefix(relRomPath, ".") {
		relRomPath = "./" + relRomPath
	}

	gameEntry := Game{
		Path:        relRomPath,
		Name:        game.GetPreferredName(g.preferredRegions),
		Desc:        game.GetPreferredSynopsis(g.preferredLanguages),
		Developer:   game.Developer.Name,
		Publisher:   game.Publisher.Name,
		Genre:       game.GetGenreNames(),
		Players:     game.Players.Text,
		ReleaseDate: formatReleaseDate(game.GetReleaseDate(g.preferredRegions)),
	}

	// Parse rating (convert to 0-1 scale if needed)
	if len(game.Ratings) > 0 {
		if rating, err := parseRating(game.Ratings[0].Value); err == nil {
			gameEntry.Rating = rating
		}
	}

	// Add media file paths (relative to ROM directory)
	for mediaType, localPath := range mediaFiles {
		relMediaPath, err := filepath.Rel(g.romDir, localPath)
		if err != nil {
			continue
		}

		// Add "./" prefix for EmulationStation
		if !strings.HasPrefix(relMediaPath, ".") {
			relMediaPath = "./" + relMediaPath
		}

		switch mediaType {
		case "box-2D", "box-texture", "screenmarquee":
			if gameEntry.Image == "" {
				gameEntry.Image = relMediaPath
			}
		case "screenshot-title":
			if gameEntry.Thumbnail == "" {
				gameEntry.Thumbnail = relMediaPath
			}
		case "video", "video-normalized":
			if gameEntry.Video == "" {
				gameEntry.Video = relMediaPath
			}
		case "wheel", "wheel-hd", "wheel-steel", "wheel-carbon":
			if gameEntry.Marquee == "" {
				gameEntry.Marquee = relMediaPath
			}
		}
	}

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

// formatReleaseDate converts various date formats to YYYYMMDD format
func formatReleaseDate(date string) string {
	// ScreenScraper returns dates in YYYY-MM-DD format
	// EmulationStation uses YYYYMMDDTHHMISS format, but YYYYMMDD is acceptable
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")

	// Ensure we have at least YYYYMMDD (8 characters)
	if len(date) >= 8 {
		return date[:8] + "T000000"
	}

	return date
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
