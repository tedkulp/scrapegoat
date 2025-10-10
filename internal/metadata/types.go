package metadata

import "encoding/xml"

// GameList represents the EmulationStation gamelist.xml structure
type GameList struct {
	XMLName xml.Name `xml:"gameList"`
	Games   []Game   `xml:"game"`
}

// Game represents a single game entry in gamelist.xml
// Note: Media tags (image, video, marquee, thumbnail) are not included
// as EmulationStation finds media files by convention based on ROM filename
type Game struct {
	Path        string  `xml:"path"`
	Name        string  `xml:"name,omitempty"`
	SortName    string  `xml:"sortname,omitempty"`
	Desc        string  `xml:"desc,omitempty"`
	Rating      float64 `xml:"rating,omitempty"`
	ReleaseDate string  `xml:"releasedate,omitempty"`
	Developer   string  `xml:"developer,omitempty"`
	Publisher   string  `xml:"publisher,omitempty"`
	Genre       string  `xml:"genre,omitempty"`
	Players     string  `xml:"players,omitempty"`
	Favorite    bool    `xml:"favorite,omitempty"`
	Hidden      bool    `xml:"hidden,omitempty"`
	KidGame     bool    `xml:"kidgame,omitempty"`
	PlayCount   int     `xml:"playcount,omitempty"`
	LastPlayed  string  `xml:"lastplayed,omitempty"`
}

// NewGameList creates a new empty GameList
func NewGameList() *GameList {
	return &GameList{
		Games: []Game{},
	}
}

// AddGame adds a game to the game list
func (gl *GameList) AddGame(game Game) {
	gl.Games = append(gl.Games, game)
}
