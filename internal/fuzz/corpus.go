// internal/fuzz/corpus.go
package fuzz

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

// Provenance addresses one generated program by (seed, bound, index), the
// FR-017 traceability tuple that lets a run's exact draw be identified later.
type Provenance struct {
	Seed         int64     `json:"seed"`
	BoundKind    BoundKind `json:"bound_kind"`
	BoundValue   int       `json:"bound_value"`
	ProgramIndex int       `json:"program_index"`
}

func (p Provenance) Filename() string {
	return fmt.Sprintf("seed%d_b%s_%d_idx%d.json", p.Seed, p.BoundKind, p.BoundValue, p.ProgramIndex)
}

type Finding struct {
	Provenance Provenance `json:"provenance"`
	Source     string     `json:"source"`
	Runs       []ShellRun `json:"runs"`
	Detail     string     `json:"detail"`
	CarvedZsh  bool       `json:"carved_zsh"`
	Shrunk     bool       `json:"shrunk"`
	// Unstable distinguishes the FR-008 recompile-instability oracle (same source
	// compiled twice -> different bytes) from a genuine cross-shell divergence
	// (FR-007). Both are findings, but they are DIFFERENT bugs; the CLI/report label
	// them separately so an operator is not told "DIVERGENCE" for a codegen-stability
	// failure. Set from OracleResult.Unstable at persist time.
	Unstable bool `json:"unstable"`
}

// PersistCandidate writes f as JSON under dir, named by provenance so a shrunk
// version overwrites the pre-shrink one (FR-013). temp+rename for interrupt safety.
func PersistCandidate(dir string, f Finding) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, f.Provenance.Filename())
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// LoadCorpus reads every persisted Finding under dir, sorted by filename for
// stable ordering. A missing dir is treated as an empty corpus, not an error.
func LoadCorpus(dir string) ([]Finding, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []Finding
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var f Finding
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		out = append(out, f)
	}
	return out, nil
}

// ReplayEntry re-executes a corpus entry's stored source with its stored carve
// decision (FR-017: replay runs stored source, does not regenerate).
// replaySource is the oracle seam ReplayEntry dispatches through; production points at
// runOracleSource, unit tests swap it to assert the STORED source+carve are what run.
var replaySource = runOracleSource

func ReplayEntry(shells []testrunner.Shell, f Finding) (OracleResult, error) {
	return replaySource(shells, f.Source, f.CarvedZsh)
}
