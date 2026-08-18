// internal/fuzz/replay_test.go
package fuzz

import (
	"strings"
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
// committed curated regression corpus parses via LoadCorpus and every carve-clean
// entry stores a compiling source with the carve decision recorded. The detector
// reach of the stored sources is the sibling test
// TestCuratedRegressionsReachBoundaryArith; the real four-shell replay is
// TestCuratedReplayPasses (fuzzshell).
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
		if _, err := compileOnce(f.Source); err != nil {
			t.Fatalf("curated entry %s stored source does not compile: %v", f.Provenance.Filename(), err)
		}
		found = true
	}
	if !found {
		t.Fatal("no carve-clean curated entry found (expected intmin-via-var.json with CarvedZsh:true)")
	}
}

// TestCuratedRegressionsReachBoundaryArith pins every curated carve entry with a
// REAL provenance (seed != 0) to the structural detector on BOTH surfaces:
//
//  1. The original pre-shrink program regenerated from (seed, bound, index) via
//     RegenerateProgram must satisfy programReachesBoundaryArith -- the exact
//     program the manifest ran and the fuzzer shrunk from. The fuzzer has no
//     source-to-IR parser, so the regenerated IR is the only faithful anchor for
//     the detector; a future detector over-narrowing fails this test instead of
//     silently passing CI and the fuzzshell replay (which use the STORED flag).
//     A generator (gen.go/rng.go/builtins.go) drift would repoint regeneration at
//     a different program -- that is acceptable for this pin: the SC-005 baseline
//     test re-runs the manifest through the current generator, and the stored-
//     source guard in (2) still bounds the replay surface.
//
//  2. The stored SHRUNK source replay actually executes must still carry a
//     boundary marker (literal or math.int_min/int_max call) in its text -- the
//     text-level guard that holds even if the generator drifts.
//
// The intmin-via-var entry (placeholder seed 0, not a real manifest program) is
// exempt from (1); its direct min-side shape is pinned for the general detector by
// TestBoundaryArithSubsumesMinSide.
func TestCuratedRegressionsReachBoundaryArith(t *testing.T) {
	entries, err := LoadCorpus("corpus/regressions")
	if err != nil {
		t.Fatalf("load curated corpus: %v", err)
	}
	markers := []string{intMinLiteral, intMaxLiteral, "math.int_min", "math.int_max"}
	hasMarker := func(src string) bool {
		for _, m := range markers {
			if strings.Contains(src, m) {
				return true
			}
		}
		return false
	}
	checked := 0
	for _, f := range entries {
		if !f.CarvedZsh {
			continue
		}
		if !hasMarker(f.Source) {
			t.Errorf("curated entry %s: stored source carries no boundary marker; the carve decision would be stale", f.Provenance.Filename())
		}
		if f.Provenance.Seed == 0 {
			continue // placeholder provenance (intmin-via-var); exempt from regeneration
		}
		p, err := RegenerateProgram(f.Provenance, DefaultGenConfig())
		if err != nil {
			t.Fatalf("regenerate %s: %v", f.Provenance.Filename(), err)
		}
		if !programReachesBoundaryArith(p) {
			t.Errorf("curated entry %s: regenerated program does not reach boundary arithmetic; the stored carve decision would go stale", f.Provenance.Filename())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no curated carve entries with real provenance found to pin")
	}
}
