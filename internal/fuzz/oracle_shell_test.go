// internal/fuzz/oracle_shell_test.go
//go:build fuzzshell

package fuzz

import "testing"

func TestOracleCleanOnCorrectCompiler(t *testing.T) {
	shells, err := RequirePrereqs()
	if err != nil {
		t.Skipf("prereqs unavailable: %v", err)
	}
	for i := 0; i < 100; i++ {
		p := Generate(NewRNG(int64(i)), DefaultGenConfig())
		res, err := RunOracle(shells, p)
		if err != nil {
			t.Fatalf("seed %d oracle error: %v\n%s", i, err, Print(p))
		}
		if res.Diverged {
			t.Fatalf("seed %d DIVERGED on the correct compiler: %s\n%s", i, res.Detail, Print(p))
		}
		if res.Unstable {
			t.Fatalf("seed %d recompile-unstable: %s\n%s", i, res.Detail, Print(p))
		}
	}
}
