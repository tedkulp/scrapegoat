package metadata

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// rawGame is a <game> element kept exactly as it was read: its attributes
// and its inner XML, including tags Scrapegoat does not know.
type rawGame struct {
	XMLName xml.Name   `xml:"game"`
	Attrs   []xml.Attr `xml:",any,attr"`
	Inner   string     `xml:",innerxml"`
}

// Existing is the entries of a gamelist.xml written before this run, by path.
type Existing struct {
	byPath map[string]rawGame
}

// ReadExisting reads the gamelist.xml at path so that entries can be carried
// over into a new one. A missing file has no entries and is not an error.
func ReadExisting(path string) (*Existing, error) {
	e := &Existing{byPath: map[string]rawGame{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return e, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var list struct {
		Games []rawGame `xml:"game"`
	}
	if err := xml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, g := range list.Games {
		var p struct {
			Path string `xml:"path"`
		}
		if err := xml.Unmarshal([]byte("<game>"+g.Inner+"</game>"), &p); err != nil || p.Path == "" {
			continue
		}
		key := pathKey(p.Path)
		if _, dup := e.byPath[key]; !dup {
			e.byPath[key] = g
		}
	}
	return e, nil
}

// pathKey normalizes a gamelist <path> so "./A.nes" and "A.nes" match.
func pathKey(p string) string { return path.Clean(filepath.ToSlash(p)) }

// CarryOver adds the existing entry for the ROM at romPath to the gamelist
// unchanged, matched by <path> (the same path AddGameFromScraper writes). It reports
// whether there was an entry to carry over.
func (g *Generator) CarryOver(existing *Existing, romPath string) bool {
	if existing == nil {
		return false
	}
	entry, ok := existing.byPath[pathKey(entryPath(romPath))]
	if ok {
		g.entries = append(g.entries, entry)
	}
	return ok
}
