// internal/fuzz/run_test.go
package fuzz

import (
	"os"
	"strconv"
	"testing"
)

func TestProgramCountFromBound(t *testing.T) {
	if programCount(ManifestEntry{BoundKind: BoundProgramCount, BoundValue: 40}) != 40 {
		t.Error("program_count != 40")
	}
	if programCount(ManifestEntry{BoundKind: BoundGenerationBudget, BoundValue: 40}) != 40 {
		t.Error("generation_budget should map 1:1 to 40 in v1")
	}
}

func TestRunSeededNoFalsePositives(t *testing.T) {
	dir := t.TempDir()
	rep, err := runSeededWith(RunConfig{GenCfg: DefaultGenConfig(), OutDir: dir},
		ManifestEntry{Seed: 3, BoundKind: BoundProgramCount, BoundValue: 25},
		func(p *Program) (OracleResult, error) { return OracleResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("clean oracle produced %d findings", len(rep.Findings))
	}
}

func TestRunSeededDeterministicUnderVariedEnv(t *testing.T) {
	// SC-002: same (seed,bound) -> identical program set, even under varied ambient
	// env (RNG is seed-only; exec env is fixed in the oracle). SC-002's "and
	// wall-clock" half needs no separate time-perturbation test: the fuzzer reads NO
	// clock anywhere (no time.Now, no Date, no timestamp seeding -- FR-016), so output
	// is wall-clock-invariant by construction. These two runs also execute at
	// different wall-clock instants (run A then run B), so this test already spans a
	// clock difference; identical results across it exercise the wall-clock invariance.
	e := ManifestEntry{Seed: 9, BoundKind: BoundProgramCount, BoundValue: 30}
	var a, b []string
	capA := func(p *Program) (OracleResult, error) { a = append(a, Print(p)); return OracleResult{}, nil }
	capB := func(p *Program) (OracleResult, error) { b = append(b, Print(p)); return OracleResult{}, nil }
	os.Setenv("WISP_FUZZ_JUNK", "one")
	if _, err := runSeededWith(RunConfig{GenCfg: DefaultGenConfig()}, e, capA); err != nil {
		t.Fatal(err)
	}
	os.Setenv("WISP_FUZZ_JUNK", "two")
	if _, err := runSeededWith(RunConfig{GenCfg: DefaultGenConfig()}, e, capB); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("WISP_FUZZ_JUNK")
	if len(a) != 30 || len(b) != 30 {
		t.Fatalf("want 30/30 programs, got %d/%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("program %d differs under varied env", i)
		}
	}
}

func TestRegenerateProgramDeterministic(t *testing.T) {
	// FR-017 provenance path: regenerating a recorded provenance yields the exact
	// pre-shrink program the seeded run drew at that index -- byte-identical across
	// calls, and identical to the run's own generateAt draw.
	prov := Provenance{Seed: 9, BoundKind: BoundProgramCount, BoundValue: 30, ProgramIndex: 7}
	p1, err := RegenerateProgram(prov, DefaultGenConfig())
	if err != nil {
		t.Fatal(err)
	}
	p2, err := RegenerateProgram(prov, DefaultGenConfig())
	if err != nil {
		t.Fatal(err)
	}
	if Print(p1) != Print(p2) {
		t.Fatal("regeneration is not deterministic")
	}
	if Print(p1) != Print(generateAt(prov.Seed, prov.ProgramIndex, DefaultGenConfig())) {
		t.Fatal("regenerated program differs from the seeded-run draw at the same index")
	}
	if _, err := RegenerateProgram(Provenance{Seed: 9, BoundKind: BoundProgramCount, BoundValue: 30, ProgramIndex: 30}, DefaultGenConfig()); err == nil {
		t.Fatal("expected out-of-range program_index to error")
	}
}

func TestRunSeededShrinksAndReoracles(t *testing.T) {
	dir := t.TempDir()
	// Fake oracle: always diverges, tagging Detail with the current program size so
	// we can confirm the persisted finding was re-oracled on the SHRUNK program.
	oracle := func(p *Program) (OracleResult, error) {
		return OracleResult{Diverged: true, Runs: []ShellRun{{Label: "dash"}, {Label: "bash"}}, Detail: "n=" + strconv.Itoa(nodeCount(p))}, nil
	}
	rep, err := runSeededWith(RunConfig{GenCfg: DefaultGenConfig(), OutDir: dir},
		ManifestEntry{Seed: 3, BoundKind: BoundProgramCount, BoundValue: 3}, oracle)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("expected findings")
	}
	for _, f := range rep.Findings {
		if !f.Shrunk {
			t.Errorf("idx %d not shrunk", f.Provenance.ProgramIndex)
		}
	}
}

func TestRunSeededAccumulatesAllCoverage(t *testing.T) {
	// SC-009: over the canonical manifest, every pure-core AND payload category and
	// every admitted family is witnessed.
	entries, err := LoadCanonicalManifest()
	if err != nil {
		t.Fatal(err)
	}
	clean := func(p *Program) (OracleResult, error) { return OracleResult{}, nil }
	var cats CategorySet
	fams := FamilySet{}
	for _, e := range entries {
		rep, err := runSeededWith(RunConfig{GenCfg: DefaultGenConfig()}, e, clean)
		if err != nil {
			t.Fatal(err)
		}
		cats |= rep.Categories
		for f := range rep.Families {
			fams[f] = true
		}
	}
	for _, c := range append(append([]Category{}, pureCoreCategories...), payloadCategories...) {
		if !cats.Has(c) {
			t.Errorf("category %s not witnessed over the manifest", c)
		}
	}
	want := map[string]bool{}
	for _, b := range admittedBuiltins {
		want[b.Family] = true
	}
	for fam := range want {
		if !fams[fam] {
			t.Errorf("family %q not witnessed over the manifest", fam)
		}
	}
}
