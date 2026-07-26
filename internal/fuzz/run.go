// internal/fuzz/run.go
package fuzz

import (
	"fmt"

	"github.com/mitchellnemitz/wisp/internal/driver"
	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

type RunConfig struct {
	Shells []testrunner.Shell
	GenCfg GenConfig
	OutDir string
}

type RunReport struct {
	Findings   []Finding
	Categories CategorySet
	Families   FamilySet
}

// programCount maps a bound to an exact program count. v1's cost model is one
// program per work unit, so both bound kinds map 1:1 to bound_value.
func programCount(e ManifestEntry) int { return e.BoundValue }

type oracleFn func(p *Program) (OracleResult, error)

func generateAt(seed int64, index int, cfg GenConfig) *Program {
	return Generate(NewRNG(seed*1000003+int64(index)), cfg)
}

// RegenerateProgram deterministically rebuilds the exact ORIGINAL pre-shrink program
// addressed by a provenance tuple (FR-017's debugging/traceability path -- NOT the
// corpus-replay path, which re-executes a shrunk program's stored source). It is the
// same generator draw generateAt makes during a seeded/soak run, so regenerating a
// recorded provenance yields a byte-identical program (the acceptance scenario:
// "a recorded failing provenance regenerates the same divergent pre-shrink program").
// In v1's cost model both bound kinds map 1:1 to program index, so BoundKind/BoundValue
// address and disambiguate the provenance but do not alter the drawn program; they are
// validated only to reject a nonsensical index beyond the run's own bound.
func RegenerateProgram(prov Provenance, cfg GenConfig) (*Program, error) {
	if prov.ProgramIndex < 0 || prov.ProgramIndex >= prov.BoundValue {
		return nil, fmt.Errorf("program_index %d out of range for bound %s=%d", prov.ProgramIndex, prov.BoundKind, prov.BoundValue)
	}
	return generateAt(prov.Seed, prov.ProgramIndex, cfg), nil
}

// requireRunPrereqs enforces the full FR-019 preflight at the library boundary on an
// ALREADY-DISCOVERED shell set, so a caller that bypasses the CLI (which calls the
// exported RequirePrereqs) cannot silently run with a broken toolchain: (1) all four
// shells present (no reduced matrix), AND (2) the compiler actually compiles a trivial
// program (reusing compileOnce, the same smoke path RequirePrereqs uses). The compile
// smoke test catches a broken/absent codegen path up front rather than as a flood of
// per-program "compile failed" errors mid-run. runSeededWith (the injectable-oracle
// seam used by unit tests) does NOT call this, so fake-oracle tests need no real
// shells or compiler.
func requireRunPrereqs(shells []testrunner.Shell) error {
	if m := missingShells(shells); len(m) > 0 {
		return fmt.Errorf("incomplete shell matrix: missing %v (FR-019; no reduced matrix)", m)
	}
	if _, err := compileOnce("fn main() -> int {\n  print(\"${1 + 1}\")\n  return 0\n}\n"); err != nil {
		return fmt.Errorf("compiler preflight failed (FR-019): %w", err)
	}
	return nil
}

// DefaultFindingsDir is where a real run persists findings when the caller leaves
// OutDir empty. FR-013/FR-017 make persistence MANDATORY on every divergence, so the
// real-run entrypoints (RunSeeded/RunSoak) never leave OutDir empty -- a caller cannot
// silently opt out of persistence. (runSeededWith, the fake-oracle test seam, is
// exempt and honors an empty OutDir so unit tests need not write files; those tests
// never diverge with an empty OutDir.)
const DefaultFindingsDir = "internal/fuzz/corpus/findings"

func RunSeeded(cfg RunConfig, e ManifestEntry) (RunReport, error) {
	if err := requireRunPrereqs(cfg.Shells); err != nil {
		return RunReport{}, err
	}
	if cfg.OutDir == "" {
		cfg.OutDir = DefaultFindingsDir // FR-013: persistence is mandatory for real runs
	}
	return runSeededWith(cfg, e, func(p *Program) (OracleResult, error) { return RunOracle(cfg.Shells, p) })
}

func runSeededWith(cfg RunConfig, e ManifestEntry, oracle oracleFn) (RunReport, error) {
	rep := RunReport{Families: FamilySet{}}
	for i := 0; i < programCount(e); i++ {
		p := generateAt(e.Seed, i, cfg.GenCfg)
		rep.Categories |= p.Cats
		for f := range p.Fams {
			rep.Families[f] = true
		}
		res, err := oracle(p)
		if err != nil {
			return rep, err
		}
		if !res.Diverged && !res.Unstable {
			continue
		}
		prov := Provenance{Seed: e.Seed, BoundKind: e.BoundKind, BoundValue: e.BoundValue, ProgramIndex: i}
		f, err := handleDivergence(cfg, oracle, prov, p, res)
		if err != nil {
			return rep, err
		}
		rep.Findings = append(rep.Findings, f)
	}
	return rep, nil
}

// handleDivergence persists the raw finding, shrinks, RE-RUNS the oracle on the
// 1-minimal program (so its persisted per-shell outputs/detail/carve match the
// stored source -- FR-010/SC-011), then persists the shrunk record. Shared by
// seeded and soak (FR-013). Oracle errors during shrink are surfaced.
func handleDivergence(cfg RunConfig, oracle oracleFn, prov Provenance, p *Program, res OracleResult) (Finding, error) {
	raw := Finding{Provenance: prov, Source: Print(p), Runs: res.Runs, Detail: res.Detail, CarvedZsh: res.CarvedZsh, Unstable: res.Unstable}
	if cfg.OutDir != "" {
		if _, err := PersistCandidate(cfg.OutDir, raw); err != nil {
			return Finding{}, err
		}
	}
	var shrinkErr error
	diverges := func(pr *Program) bool {
		r, err := oracle(pr)
		if err != nil {
			shrinkErr = err // surface, do not swallow; reject this reduction
			return false
		}
		return r.Diverged || r.Unstable
	}
	typechecks := func(pr *Program) bool {
		_, _, diags := driver.Compile("fuzz.wisp", Print(pr))
		for _, d := range diags {
			if d.Severity == driver.Error {
				return false
			}
		}
		return true
	}
	min := Shrink(p, typechecks, diverges)
	if shrinkErr != nil {
		return Finding{}, fmt.Errorf("oracle error during shrink of %s: %w", prov.Filename(), shrinkErr)
	}
	// Re-oracle the minimal program so the stored finding is self-consistent.
	minRes, err := oracle(min)
	if err != nil {
		return Finding{}, err
	}
	shrunk := Finding{Provenance: prov, Source: Print(min), Runs: minRes.Runs, Detail: minRes.Detail, CarvedZsh: minRes.CarvedZsh, Shrunk: true, Unstable: minRes.Unstable}
	if cfg.OutDir != "" {
		if _, err := PersistCandidate(cfg.OutDir, shrunk); err != nil {
			return Finding{}, err
		}
	}
	return shrunk, nil
}

// RunSoak generates until stop is closed, handling each divergence with the same
// persist+shrink+re-oracle flow as seeded mode (FR-013).
func RunSoak(cfg RunConfig, seed int64, stop <-chan struct{}) (RunReport, error) {
	if err := requireRunPrereqs(cfg.Shells); err != nil {
		return RunReport{}, err
	}
	if cfg.OutDir == "" {
		cfg.OutDir = DefaultFindingsDir // FR-013: persistence is mandatory for real runs
	}
	rep := RunReport{Families: FamilySet{}}
	oracle := func(p *Program) (OracleResult, error) { return RunOracle(cfg.Shells, p) }
	for i := 0; ; i++ {
		select {
		case <-stop:
			return rep, nil
		default:
		}
		p := generateAt(seed, i, cfg.GenCfg)
		rep.Categories |= p.Cats
		for f := range p.Fams {
			rep.Families[f] = true
		}
		res, err := oracle(p)
		if err != nil {
			return rep, err
		}
		if !res.Diverged && !res.Unstable {
			continue
		}
		prov := Provenance{Seed: seed, BoundKind: BoundProgramCount, BoundValue: i + 1, ProgramIndex: i}
		f, err := handleDivergence(cfg, oracle, prov, p, res)
		if err != nil {
			return rep, err
		}
		rep.Findings = append(rep.Findings, f)
	}
}
