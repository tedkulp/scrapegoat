package artwork

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tedkulp/scrapegoat/internal/media"
)

// MediaTypes are the media type folder names a layer resource can use: the
// EmulationStation media folders internal/media places media in.
var MediaTypes = media.Types()

// CustomResourcePrefix marks a layer resource as a file in resources_dir.
const CustomResourcePrefix = "custom:"

// Validate checks the artwork outputs: every layer resource must be a media
// type or a custom: resource, no output may write to a media type folder (it
// would overwrite downloaded media and read its own artwork back as a layer),
// and no two outputs may write the same format to the same folder (they would
// overwrite each other's files).
func (c Config) Validate() error {
	var errs []error

	isMediaType := make(map[string]bool, len(MediaTypes))
	for _, t := range MediaTypes {
		isMediaType[t] = true
	}

	type target struct{ dir, format string }
	firstWriter := make(map[target]int)

	for i, def := range c.Outputs {
		for _, layer := range def.Layers {
			if strings.HasPrefix(layer.Resource, CustomResourcePrefix) || isMediaType[layer.Resource] {
				continue
			}
			errs = append(errs, fmt.Errorf("artwork output %s: unknown layer resource %q (use a media type: %s; or custom:<path>)",
				describeOutput(i, def), layer.Resource, strings.Join(MediaTypes, ", ")))
		}

		t := target{dir: filepath.Clean(c.outputDirFor(def)), format: def.fileFormat()}
		if mediaType := mediaTypeFolder(t.dir); mediaType != "" {
			errs = append(errs, fmt.Errorf("artwork output %s writes to the media type folder %q, which holds downloaded media; give it its own output_dir",
				describeOutput(i, def), mediaType))
			continue
		}
		if j, ok := firstWriter[t]; ok {
			errs = append(errs, fmt.Errorf("artwork outputs %s and %s both write %s files to %q; give one of them its own output_dir",
				describeOutput(j, c.Outputs[j]), describeOutput(i, def), t.format, t.dir))
			continue
		}
		firstWriter[t] = i
	}

	return errors.Join(errs...)
}

// mediaTypeFolder returns the media type folder dir names, ignoring case, or
// "" when dir is not one.
func mediaTypeFolder(dir string) string {
	for _, t := range MediaTypes {
		if strings.EqualFold(dir, t) {
			return t
		}
	}
	return ""
}

func describeOutput(i int, def OutputDefinition) string {
	return fmt.Sprintf("#%d (%q)", i+1, def.Type)
}
