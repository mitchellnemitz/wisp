//go:build fuzzshell

// internal/fuzz/baseline_test.go
package fuzz

import (
	"bytes"
	"os"
	"testing"
)

func TestManifestBaselineClean(t *testing.T) {
	shells, err := RequirePrereqs()
	if err != nil {
		t.Skipf("prereqs unavailable: %v", err)
	}
	entries, err := LoadCanonicalManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		rep, err := RunSeeded(RunConfig{Shells: shells, GenCfg: DefaultGenConfig()}, e)
		if err != nil {
			t.Fatalf("seed %d: %v", e.Seed, err)
		}
		if len(rep.Findings) != 0 {
			t.Fatalf("seed %d produced %d findings on the correct compiler: %s", e.Seed, len(rep.Findings), rep.Findings[0].Detail)
		}
	}
}

// TestOracleDeterministicUnderVariedEnv is the REAL-ORACLE half of SC-002: the same
// (seed,bound) run twice under different ambient env yields byte-identical per-shell
// outputs, exit statuses, and carve decisions -- not just identical source (that is
// the CI-side TestRunSeededDeterministicUnderVariedEnv). This proves the fuzzer adds
// no nondeterminism of its own end-to-end, through actual shell execution (FR-016).
func TestOracleDeterministicUnderVariedEnv(t *testing.T) {
	shells, err := RequirePrereqs()
	if err != nil {
		t.Skipf("prereqs unavailable: %v", err)
	}
	run := func() []OracleResult {
		var rs []OracleResult
		for i := 0; i < 25; i++ {
			p := generateAt(7, i, DefaultGenConfig())
			r, err := RunOracle(shells, p)
			if err != nil {
				t.Fatalf("oracle error at idx %d: %v", i, err)
			}
			rs = append(rs, r)
		}
		return rs
	}
	os.Setenv("WISP_FUZZ_JUNK", "alpha")
	a := run()
	os.Setenv("WISP_FUZZ_JUNK", "beta")
	b := run()
	os.Unsetenv("WISP_FUZZ_JUNK")
	if len(a) != len(b) {
		t.Fatalf("run length differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Diverged != b[i].Diverged || a[i].Unstable != b[i].Unstable || a[i].CarvedZsh != b[i].CarvedZsh || len(a[i].Runs) != len(b[i].Runs) {
			t.Fatalf("result %d differs under varied env: %+v vs %+v", i, a[i], b[i])
		}
		for j := range a[i].Runs {
			if a[i].Runs[j].Exit != b[i].Runs[j].Exit || !bytes.Equal(a[i].Runs[j].Stdout, b[i].Runs[j].Stdout) {
				t.Fatalf("result %d shell %s differs under varied env", i, a[i].Runs[j].Label)
			}
		}
	}
}
