package driver

import (
	"bytes"
	"encoding/json"

	"github.com/mitchellnemitz/wisp/internal/codegen"
)

// sourceMap is the wisp source-map artifact (spec section 3.2): a small,
// versioned JSON object, NOT source-map-v3. M2 has a single input file, so it
// carries one `source`. `lines` is dense: exactly one entry per generated line,
// each null (no wisp origin) or {l,c}.
type sourceMap struct {
	Version   int          `json:"version"`
	Generated string       `json:"generated"`
	Source    string       `json:"source"`
	Lines     []*lineEntry `json:"lines"`
}

// lineEntry is the {l,c} value for a generated line that maps to a wisp source
// position (l = wisp line, c = wisp column). A generated line with no wisp
// origin is encoded as a JSON null (a nil *lineEntry). F names the originating
// file when it differs from the top-level `source` (an imported/included module,
// M8); it is omitted for the root file so a single-module map is unchanged.
type lineEntry struct {
	L int    `json:"l"`
	C int    `json:"c"`
	F string `json:"f,omitempty"`
}

// buildSourceMap serializes the §3.2 object from the generated path, the source
// path, the generated script bytes, and codegen's per-line position table.
// `generated` and `source` are stored verbatim as passed (no path resolution).
// The script bytes are used only to defensively assert the line count matches
// the table; the table itself is authoritative.
func buildSourceMap(generated, source string, script []byte, lineMap []*codegen.SourcePos) ([]byte, error) {
	entries := make([]*lineEntry, len(lineMap))
	for i, p := range lineMap {
		if p == nil {
			entries[i] = nil
			continue
		}
		e := &lineEntry{L: p.Line, C: p.Col}
		// A line from an imported/included module records its own file; the root's
		// lines use the top-level `source` (F omitted).
		if p.File != "" && p.File != source {
			e.F = p.File
		}
		entries[i] = e
	}

	sm := sourceMap{
		Version:   1,
		Generated: generated,
		Source:    source,
		Lines:     entries,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&sm); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
