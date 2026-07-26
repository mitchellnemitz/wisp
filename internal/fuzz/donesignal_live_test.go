//go:build fuzzshell

// internal/fuzz/donesignal_live_test.go
package fuzz

import "testing"

func TestIntMinArithCleanUnderCarveOut(t *testing.T) {
	shells, err := RequirePrereqs()
	if err != nil {
		t.Skipf("prereqs unavailable: %v", err)
	}
	p := &Program{
		Imports: []string{"math"},
		Body: []Stmt{
			&LetStmt{Name: "m", T: Type{Kind: KInt}, Init: &Call{Builtin: "math.int_min", T: Type{Kind: KInt}}},
			&LetStmt{Name: "s", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &Var{Name: "m", T: Type{Kind: KInt}}, R: &IntLit{V: "0"}, T: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Var{Name: "s", T: Type{Kind: KInt}}},
		},
	}
	if !programReachesIntMinArith(p) {
		t.Fatal("detector missed INT_MIN via variable into arithmetic")
	}
	res, err := RunOracle(shells, p)
	if err != nil {
		t.Fatalf("oracle error: %v", err)
	}
	if !res.CarvedZsh {
		t.Fatal("expected the zsh carve-out to fire")
	}
	if res.Diverged {
		t.Fatalf("INT_MIN construct diverged on the correct compiler under carve-out: %s", res.Detail)
	}
}
