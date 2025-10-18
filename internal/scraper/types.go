package scraper

import (
	"encoding/xml"
	"strings"
	"time"
)

// APIResponse represents the root XML response from ScreenScraper API
type APIResponse struct {
	XMLName xml.Name `xml:"Data"`
	Header  Header   `xml:"ssuser"`
	Game    *Game    `xml:"jeu"`
}

// GameInfoJSONResponse represents the JSON response from jeuInfos.php
type GameInfoJSONResponse struct {
	Response struct {
		SSUser UserInfo `json:"ssuser"`
		Game   *Game    `json:"jeu"`
	} `json:"response"`
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
	ID        string    `xml:"id" json:"id"`
	Names     []Name    `xml:"noms>nom" json:"noms"`
	CloneOf   string    `xml:"cloneof" json:"cloneof"`
	ROM       ROM       `xml:"rom" json:"rom"`
	Medias    []Media   `xml:"medias>media" json:"medias"`
	Synopsis  []Text    `xml:"synopsis>synopsis" json:"synopsis"`
	Dates     []Date    `xml:"dates>date" json:"dates"`
	Genres    []Genre   `xml:"genres>genre" json:"genres"`
	Ratings   []Rating  `xml:"notes>note" json:"notes"`
	Developer Developer `xml:"developpeur" json:"developpeur"`
	Publisher Publisher `xml:"editeur" json:"editeur"`
	Players   Players   `xml:"joueurs" json:"joueurs"`
}

// Name represents game name in different regions
type Name struct {
	Region string `xml:"region,attr" json:"region"`
	Text   string `xml:",chardata" json:"text"`
}

// ROM contains ROM file metadata
type ROM struct {
	ID       string `xml:"id,attr" json:"id"`
	Type     string `xml:"romtype,attr" json:"romtype"`
	Filename string `xml:"romfilename,attr" json:"romfilename"`
	CRC      string `xml:"romcrc,attr" json:"romcrc"`
	MD5      string `xml:"rommd5,attr" json:"rommd5"`
	SHA1     string `xml:"romsha1,attr" json:"romsha1"`
	Size     string `xml:"romsize,attr" json:"romsize"`
}

// Media represents downloadable media (images, videos, etc.)
type Media struct {
	Type   string `xml:"type,attr" json:"type"`
	Region string `xml:"region,attr" json:"region"`
	Format string `xml:"format,attr" json:"format"`
	URL    string `xml:",chardata" json:"url"`
}

// Text represents localized text (synopsis, descriptions, etc.)
type Text struct {
	Language string `xml:"langue,attr" json:"langue"`
	Text     string `xml:",chardata" json:"text"`
}

// Date represents release date information
type Date struct {
	Region string `xml:"region,attr" json:"region"`
	Text   string `xml:",chardata" json:"text"`
}

// Genre represents game genre
type Genre struct {
	ID   string `xml:"id,attr" json:"id"`
	Name string `xml:",chardata" json:"text"`
}

// Rating represents game rating from different sources
type Rating struct {
	Type  string `xml:"type,attr" json:"type"`
	Value string `xml:",chardata" json:"text"`
}

// Developer represents game developer
type Developer struct {
	ID   string `xml:"id,attr" json:"id"`
	Name string `xml:",chardata" json:"text"`
}

// Publisher represents game publisher
type Publisher struct {
	ID   string `xml:"id,attr" json:"id"`
	Name string `xml:",chardata" json:"text"`
}

// Players represents number of players
type Players struct {
	Text string `xml:",chardata" json:"text"`
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

// GetFirstGenre returns only the first genre name
func (g *Game) GetFirstGenre() string {
	if len(g.Genres) == 0 {
		return ""
	}

	return g.Genres[0].Name
}

// IsNonGame checks if the game is marked as a non-game (e.g., demos, applications, etc.)
// ScreenScraper marks these with names like "ZZZ(notgame):#NONGAME"
func (g *Game) IsNonGame() bool {
	// Check all name variants for the NONGAME marker
	for _, name := range g.Names {
		if strings.Contains(name.Text, "#NONGAME") || strings.Contains(name.Text, "notgame") {
			return true
		}
	}
	return false
}

// SystemsListResponse represents the XML response from systemesListe.php
type SystemsListResponse struct {
	XMLName xml.Name `xml:"Data"`
	Systems []System `xml:"systemes>systeme"`
}

// SystemsListJSONResponse represents the JSON response from systemesListe.php
type SystemsListJSONResponse struct {
	Response struct {
		Systems []SystemJSON `json:"systemes"`
	} `json:"response"`
}

// UserInfoJSONResponse represents the JSON response from ssuserInfos.php
type UserInfoJSONResponse struct {
	Response struct {
		SSUser UserInfo `json:"ssuser"`
	} `json:"response"`
}

// UserInfo represents user account information and API quota details
type UserInfo struct {
	ID                   string `json:"id"`
	NumeroID             string `json:"numeroid"`
	Level                string `json:"niveau"`
	Contribution         string `json:"contribution"`
	Uploads              string `json:"uploadsysteme"`
	ROMsAssociated       string `json:"romasso"`
	PropositionsAccepted string `json:"propositionok"`
	PropositionsRefused  string `json:"propositionko"`
	PropositionsPending  string `json:"propositionenattente"`
	Favs                 string `json:"favregion"`
	MaxThreads           string `json:"maxthreads"`
	RequestsToday        string `json:"requeststoday"`
	MaxRequestsPerDay    string `json:"maxrequestsperday"`
	MaxRequestsPerHour   string `json:"maxrequestskohour"`
	MaxRequestsPerMinute string `json:"maxrequestsperminute"`
	MaxRequestsPerSecond string `json:"maxrequestspersecondes"`
	VisitorIP            string `json:"visitorip"`
}

// GameCacheEntry represents a cached game info entry
type GameCacheEntry struct {
	FetchedAt  time.Time `json:"fetched_at"`
	Game       *Game     `json:"game"`
	NotFound   bool      `json:"not_found"`   // True if ROM was not found in API
	IsNonGame  bool      `json:"is_non_game"` // True if ROM is marked as non-game
}

// IsExpired checks if the cache entry is older than the specified duration
func (g *GameCacheEntry) IsExpired(expirationHours int) bool {
	if expirationHours <= 0 {
		expirationHours = 12
	}
	return time.Since(g.FetchedAt) > time.Duration(expirationHours)*time.Hour
}

// SystemJSON represents a gaming platform/system from ScreenScraper (JSON format)
type SystemJSON struct {
	ID    int `json:"id"`
	Names struct {
		US     string `json:"nom_us"`
		EU     string `json:"nom_eu"`
		Common string `json:"noms_commun"`
	} `json:"noms"`
	Extensions string `json:"extensions"` // Comma-separated list
}

// System represents a gaming platform/system from ScreenScraper (unified format)
type System struct {
	ID         int
	Names      []Name
	Extensions []string
}

// GetPreferredName returns the system name for the specified region, falling back to others
func (s *System) GetPreferredName(preferredRegions []string) string {
	// Try preferred regions in order
	for _, region := range preferredRegions {
		for _, name := range s.Names {
			if name.Region == region {
				return name.Text
			}
		}
	}

	// Fall back to first available name
	if len(s.Names) > 0 {
		return s.Names[0].Text
	}

	return ""
}
