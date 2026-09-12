package docs

import (
	"os"
	"path/filepath"

	"lessmess/internal/model"
)

// DocsContent is one directory's docs content for UI consumption: the
// purpose line and entry blurbs carried by its STRUCTURE.md.
type DocsContent struct {
	Purpose string            // one-line directory purpose ("" when still a placeholder)
	Blurbs  map[string]string // entry base name -> purpose blurb (placeholders omitted)
	Meta    *model.DocMeta    // freshness metadata (nil when absent)
}

// DirDocs reads d's STRUCTURE.md and returns its content. A missing file or
// one without a marker section yields zero values without error; corrupt
// markers or bad metadata are errors.
func DirDocs(d *Dir) (*DocsContent, error) {
	data, err := os.ReadFile(filepath.Join(d.Abs, StructureFile))
	if os.IsNotExist(err) {
		return &DocsContent{Blurbs: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	purpose, blurbs, meta, err := carryForward(d.Rel, data)
	if err != nil {
		return nil, err
	}
	if purpose == Placeholder {
		purpose = ""
	}
	return &DocsContent{Purpose: purpose, Blurbs: blurbs, Meta: meta}, nil
}
