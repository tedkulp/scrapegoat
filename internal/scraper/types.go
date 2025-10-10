package scraper

import "encoding/xml"

// APIResponse represents the root response from ScreenScraper API
type APIResponse struct {
	XMLName xml.Name `xml:"Data"`
	Header  Header   `xml:"ssuser"`
	Game    *Game    `xml:"jeu"`
}

// Header contains API response metadata and user info
type Header struct {
	ID            string `xml:"id"`
	NumeroID      string `xml:"numeroid"`
	MaxThreads    string `xml:"maxthreads"`
	RequestsToday string `xml:"requeststoday"`
	MaxRequests   string `xml:"maxrequestskohour"`
}

// Game represents game information from ScreenScraper
type Game struct {
	ID        string    `xml:"id"`
	Names     []Name    `xml:"noms>nom"`
	CloneOf   string    `xml:"cloneof"`
	ROM       ROM       `xml:"rom"`
	Medias    []Media   `xml:"medias>media"`
	Synopsis  []Text    `xml:"synopsis>synopsis"`
	Dates     []Date    `xml:"dates>date"`
	Genres    []Genre   `xml:"genres>genre"`
	Ratings   []Rating  `xml:"notes>note"`
	Developer Developer `xml:"developpeur"`
	Publisher Publisher `xml:"editeur"`
	Players   Players   `xml:"joueurs"`
}

// Name represents game name in different regions
type Name struct {
	Region string `xml:"region,attr"`
	Text   string `xml:",chardata"`
}

// ROM contains ROM file metadata
type ROM struct {
	ID       string `xml:"id,attr"`
	Type     string `xml:"romtype,attr"`
	Filename string `xml:"romfilename,attr"`
	CRC      string `xml:"romcrc,attr"`
	MD5      string `xml:"rommd5,attr"`
	SHA1     string `xml:"romsha1,attr"`
	Size     string `xml:"romsize,attr"`
}

// Media represents downloadable media (images, videos, etc.)
type Media struct {
	Type   string `xml:"type,attr"`
	Region string `xml:"region,attr"`
	Format string `xml:"format,attr"`
	URL    string `xml:",chardata"`
}

// Text represents localized text (synopsis, descriptions, etc.)
type Text struct {
	Language string `xml:"langue,attr"`
	Text     string `xml:",chardata"`
}

// Date represents release date information
type Date struct {
	Region string `xml:"region,attr"`
	Text   string `xml:",chardata"`
}

// Genre represents game genre
type Genre struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

// Rating represents game rating from different sources
type Rating struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// Developer represents game developer
type Developer struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

// Publisher represents game publisher
type Publisher struct {
	ID   string `xml:"id,attr"`
	Name string `xml:",chardata"`
}

// Players represents number of players
type Players struct {
	Text string `xml:",chardata"`
}

// GetPreferredName returns the game name for the specified region, falling back to others
func (g *Game) GetPreferredName(preferredRegions []string) string {
	// Try preferred regions in order
	for _, region := range preferredRegions {
		for _, name := range g.Names {
			if name.Region == region {
				return name.Text
			}
		}
	}

	// Fall back to first available name
	if len(g.Names) > 0 {
		return g.Names[0].Text
	}

	return ""
}

// GetPreferredSynopsis returns synopsis in preferred language
func (g *Game) GetPreferredSynopsis(preferredLanguages []string) string {
	// Try preferred languages in order
	for _, lang := range preferredLanguages {
		for _, synopsis := range g.Synopsis {
			if synopsis.Language == lang {
				return synopsis.Text
			}
		}
	}

	// Fall back to first available synopsis
	if len(g.Synopsis) > 0 {
		return g.Synopsis[0].Text
	}

	return ""
}

// GetMediaByType returns all media of a specific type
func (g *Game) GetMediaByType(mediaType string) []Media {
	var result []Media
	for _, media := range g.Medias {
		if media.Type == mediaType {
			result = append(result, media)
		}
	}
	return result
}

// GetPreferredMedia returns media of specified type, preferring certain regions
func (g *Game) GetPreferredMedia(mediaType string, preferredRegions []string) *Media {
	medias := g.GetMediaByType(mediaType)

	// Try preferred regions in order
	for _, region := range preferredRegions {
		for _, media := range medias {
			if media.Region == region {
				return &media
			}
		}
	}

	// Fall back to first available
	if len(medias) > 0 {
		return &medias[0]
	}

	return nil
}

// GetReleaseDate returns the release date for preferred region
func (g *Game) GetReleaseDate(preferredRegions []string) string {
	// Try preferred regions in order
	for _, region := range preferredRegions {
		for _, date := range g.Dates {
			if date.Region == region {
				return date.Text
			}
		}
	}

	// Fall back to first available
	if len(g.Dates) > 0 {
		return g.Dates[0].Text
	}

	return ""
}

// GetGenreNames returns all genre names as a comma-separated string
func (g *Game) GetGenreNames() string {
	if len(g.Genres) == 0 {
		return ""
	}

	result := g.Genres[0].Name
	for i := 1; i < len(g.Genres); i++ {
		result += ", " + g.Genres[i].Name
	}

	return result
}
