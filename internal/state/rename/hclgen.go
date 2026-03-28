package rename

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"
)

//go:embed templates/moved_block.tmpl
var movedTemplateFS embed.FS

var movedTmpl = template.Must(
	template.New("moved_block.tmpl").
		ParseFS(movedTemplateFS, "templates/moved_block.tmpl"),
)

type movedData struct {
	Pairs []RenamePair
}

// GenerateMovedFile returns HCL bytes containing one moved {} block per pair.
// Pairs are sorted by From address for deterministic output.
func GenerateMovedFile(pairs []RenamePair) []byte {
	if len(pairs) == 0 {
		return nil
	}

	sorted := make([]RenamePair, len(pairs))
	copy(sorted, pairs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].From < sorted[j].From
	})

	var buf bytes.Buffer
	if err := movedTmpl.Execute(&buf, movedData{Pairs: sorted}); err != nil {
		panic("rename: render moved template: " + err.Error())
	}
	return buf.Bytes()
}

// WriteMovedFile writes data to the specified path. If outputPath is empty,
// it defaults to <dir>/moved.tf. Returns the path written.
// Returns an error if the file already exists and force is false.
func WriteMovedFile(dir, outputPath string, data []byte, force bool) (string, error) {
	path := outputPath
	if path == "" {
		path = filepath.Join(dir, "moved.tf")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("%s already exists — remove it or use --force to overwrite", filepath.Base(path))
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("rename: write moved file: %w", err)
	}
	return path, nil
}
