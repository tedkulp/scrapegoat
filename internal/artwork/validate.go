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
// type or a custom: resource, and no two outputs may write the same format to
// the same folder (they would overwrite each other's files).
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
		if j, ok := firstWriter[t]; ok {
			errs = append(errs, fmt.Errorf("artwork outputs %s and %s both write %s files to %q; give one of them its own output_dir",
				describeOutput(j, c.Outputs[j]), describeOutput(i, def), t.format, t.dir))
			continue
		}
		firstWriter[t] = i
	}

	return errors.Join(errs...)
}

func describeOutput(i int, def OutputDefinition) string {
	return fmt.Sprintf("#%d (%q)", i+1, def.Type)
}
