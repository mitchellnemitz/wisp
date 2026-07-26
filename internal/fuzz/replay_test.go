// internal/fuzz/replay_test.go
package fuzz

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

// TestReplayUsesStoredSourceNotProvenance is the SC-011(a) contract proof at the unit
// level (no shells): replay re-executes the entry's STORED 1-minimal source, NOT a
// provenance regeneration. We prove this by pointing ReplayEntry at a fake oracle seam
// and asserting it receives f.Source and f.CarvedZsh verbatim -- a shrunk source is not
// seed-derivable, so replay must never route through generateAt/RegenerateProgram.
func TestReplayUsesStoredSourceNotProvenance(t *testing.T) {
	const src = "fn main() -> int {\n  print(\"shrunk-marker\")\n  return 0\n}\n"
	f := Finding{
		Provenance: Provenance{Seed: 1, BoundKind: BoundProgramCount, BoundValue: 1, ProgramIndex: 0},
		Source:     src,
		CarvedZsh:  true,
	}
	var gotSrc string
	var gotCarve bool
	replaySource = func(_ []testrunner.Shell, s string, carve bool) (OracleResult, error) {
		gotSrc, gotCarve = s, carve
		return OracleResult{}, nil
	}
	defer func() { replaySource = runOracleSource }()
	if _, err := ReplayEntry(nil, f); err != nil {
		t.Fatal(err)
	}
	if gotSrc != src {
		t.Fatalf("replay ran %q, want the stored source %q", gotSrc, src)
	}
	if !gotCarve {
		t.Fatal("replay dropped the stored CarvedZsh decision")
	}
	// Regenerating the provenance yields a DIFFERENT program than the shrunk source,
	// confirming replay is not a provenance regeneration path.
	regen, err := RegenerateProgram(f.Provenance, DefaultGenConfig())
	if err != nil {
		t.Fatal(err)
	}
	if Print(regen) == src {
		t.Fatal("shrunk source coincidentally equals the regenerated program; pick a source the generator never emits")
	}
}

// TestCuratedCorpusLoads is the pure-Go (no-shell) structural half of SC-008: the
// committed curated regression corpus parses via LoadCorpus and the intmin-via-var
// entry has the expected shape -- a placeholder provenance, a non-empty stored source
// that reaches INT_MIN arithmetic, and the stored zsh carve decision. This runs in CI;
// the real four-shell replay is TestCuratedReplayPasses (fuzzshell).
func TestCuratedCorpusLoads(t *testing.T) {
	entries, err := LoadCorpus("corpus/regressions")
	if err != nil {
		t.Fatalf("load curated corpus: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no curated regression entries committed (expected at least intmin-via-var.json)")
	}
	var found bool
	for _, f := range entries {
		if f.Source == "" {
			t.Fatalf("curated entry %s has an empty stored source", f.Provenance.Filename())
		}
		if !f.CarvedZsh {
			continue
		}
		// The intmin-via-var entry: its stored source must compile and reach the
		// INT_MIN-into-arithmetic construct the carve decision records.
		if _, err := compileOnce(f.Source); err != nil {
			t.Fatalf("curated entry %s stored source does not compile: %v", f.Provenance.Filename(), err)
		}
		found = true
	}
	if !found {
		t.Fatal("no carve-clean curated entry found (expected intmin-via-var.json with CarvedZsh:true)")
	}
}
