// internal/fuzz/oracle_test.go
package fuzz

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

func TestIntMinDetectorDataFlow(t *testing.T) {
	// direct literal operand
	if !programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "v", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &IntLit{V: intMinLiteral}, R: &IntLit{V: "0"}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("direct INT_MIN operand not detected")
	}
	// via variable binding (the R11 live-test shape)
	if !programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "m", T: Type{Kind: KInt}, Init: &Call{Builtin: "math.int_min", T: Type{Kind: KInt}}},
		&LetStmt{Name: "s", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &Var{Name: "m", T: Type{Kind: KInt}}, R: &IntLit{V: "0"}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("INT_MIN via variable binding not detected")
	}
	// INT_MIN present but never in arithmetic -> not carved
	if programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "m", T: Type{Kind: KInt}, Init: &IntLit{V: intMinLiteral}},
		&PrintStmt{Arg: &Var{Name: "m", T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("bare INT_MIN (no arithmetic) wrongly flagged")
	}
}

// TestIntMinDetectorNarrow pins the FR-015 "narrowest carve" boundary: shapes that
// must NOT carve, because carving them would mask a real zsh regression.
func TestIntMinDetectorNarrow(t *testing.T) {
	// INT_MIN literal fed ONLY to a builtin (not a source-level $(( )) operand):
	// abs/min/etc. either abort uniformly or lower to `[ -lt ]` tests. No carve.
	if programReachesIntMinArith(&Program{Body: []Stmt{
		&PrintStmt{Arg: &Call{Builtin: "math.abs", Args: []Expr{&IntLit{V: intMinLiteral}}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("INT_MIN into a builtin wrongly flagged (carve too wide -- would mask real bugs)")
	}
	// INT_MIN bound to a var, then fed ONLY to a builtin. Still no arithmetic operand.
	if programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "m", T: Type{Kind: KInt}, Init: &Call{Builtin: "math.int_min", T: Type{Kind: KInt}}},
		&PrintStmt{Arg: &Call{Builtin: "math.abs", Args: []Expr{&Var{Name: "m", T: Type{Kind: KInt}}}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("INT_MIN var into a builtin wrongly flagged (carve too wide)")
	}
	// Plain non-INT_MIN arithmetic -> never carved.
	if programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "v", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &IntLit{V: "1"}, R: &IntLit{V: "2"}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("plain int arithmetic wrongly flagged")
	}
	// A non-INT_MIN value bound to a var, then arithmetic on that var -> no carve.
	if programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "a", T: Type{Kind: KInt}, Init: &IntLit{V: "5"}},
		&LetStmt{Name: "s", T: Type{Kind: KInt}, Init: &Binary{Op: "*", L: &Var{Name: "a", T: Type{Kind: KInt}}, R: &IntLit{V: "3"}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("non-INT_MIN variable arithmetic wrongly flagged")
	}
	// A binding to INT_MIN only as an alias of another INT_MIN var, then arithmetic:
	// transitive alias flow MUST carve (the operand still reaches $(( ))).
	if !programReachesIntMinArith(&Program{Body: []Stmt{
		&LetStmt{Name: "m", T: Type{Kind: KInt}, Init: &IntLit{V: intMinLiteral}},
		&LetStmt{Name: "n", T: Type{Kind: KInt}, Init: &Var{Name: "m", T: Type{Kind: KInt}}},
		&LetStmt{Name: "s", T: Type{Kind: KInt}, Init: &Binary{Op: "-", L: &Var{Name: "n", T: Type{Kind: KInt}}, R: &IntLit{V: "1"}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("transitive INT_MIN alias into arithmetic not detected")
	}
	// INT_MIN operand nested inside a builtin's argument (source-level $(( )) still
	// present) MUST carve: the arithmetic literal reaches the compiled `$(( ))`.
	if !programReachesIntMinArith(&Program{Body: []Stmt{
		&PrintStmt{Arg: &Call{Builtin: "math.abs", Args: []Expr{
			&Binary{Op: "+", L: &IntLit{V: intMinLiteral}, R: &IntLit{V: "0"}, T: Type{Kind: KInt}},
		}, T: Type{Kind: KInt}}},
	}}) {
		t.Fatal("INT_MIN arithmetic nested in a builtin arg not detected")
	}
}

func TestCompareRunsAgree(t *testing.T) {
	runs := []ShellRun{{Label: "dash", Stdout: []byte("42\n")}, {Label: "bash", Stdout: []byte("42\n")}, {Label: "zsh", Stdout: []byte("42\n")}}
	if d, _ := compareRuns(runs, false); d {
		t.Fatal("identical runs reported as diverged")
	}
}

func TestCompareRunsByteSensitive(t *testing.T) {
	runs := []ShellRun{{Label: "dash", Stdout: []byte("x\n")}, {Label: "bash", Stdout: []byte("x")}}
	if d, _ := compareRuns(runs, false); !d {
		t.Fatal("trailing-newline difference not detected")
	}
}

func TestCompareRunsExitStatus(t *testing.T) {
	runs := []ShellRun{{Label: "dash", Stdout: []byte("x\n"), Exit: 0}, {Label: "bash", Stdout: []byte("x\n"), Exit: 3}}
	if d, _ := compareRuns(runs, false); !d {
		t.Fatal("exit-status difference not detected")
	}
}

func TestCompareRunsZshCarveOut(t *testing.T) {
	runs := []ShellRun{{Label: "dash", Stdout: []byte("v\n")}, {Label: "bash", Stdout: []byte("v\n")}, {Label: "zsh", Stdout: []byte("different\n"), Exit: 1}}
	if d, _ := compareRuns(runs, true); d {
		t.Fatal("zsh carve-out did not exclude zsh")
	}
	runs[1].Stdout = []byte("w\n")
	if d, _ := compareRuns(runs, true); !d {
		t.Fatal("carve-out wrongly suppressed a dash-vs-bash divergence")
	}
}

func TestMissingShells(t *testing.T) {
	// SC-013 logic: with a shell absent, the completeness check names it.
	got := missingShells([]testrunner.Shell{{Label: "dash"}, {Label: "bash"}, {Label: "zsh"}})
	if len(got) != 1 || got[0] != "busybox-sh" {
		t.Fatalf("missingShells = %v, want [busybox-sh]", got)
	}
	// All four present -> nothing missing.
	if got := missingShells([]testrunner.Shell{{Label: "dash"}, {Label: "busybox-sh"}, {Label: "bash"}, {Label: "zsh"}}); len(got) != 0 {
		t.Fatalf("missingShells (complete) = %v, want []", got)
	}
}

func TestOracleRunsSingleArtifactAcrossShells(t *testing.T) {
	// SC-015: the oracle compiles ONCE and every shell executes that one on-disk
	// artifact. The execUnder seam records the path each shell was invoked with; all
	// four must be the same path (no per-shell recompile, no per-shell temp file).
	orig := execUnder
	defer func() { execUnder = orig }()
	var paths []string
	execUnder = func(sh testrunner.Shell, scriptPath string) ShellRun {
		paths = append(paths, scriptPath)
		return ShellRun{Label: sh.Label, Stdout: []byte("0\n")}
	}
	shells := []testrunner.Shell{{Label: "dash"}, {Label: "busybox-sh"}, {Label: "bash"}, {Label: "zsh"}}
	if _, err := runOracleSource(shells, "fn main() -> int {\n  print(\"0\")\n  return 0\n}\n", false); err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(shells) {
		t.Fatalf("ran %d shells, want %d", len(paths), len(shells))
	}
	for i, p := range paths {
		if p != paths[0] {
			t.Fatalf("shell %d ran a different artifact path %q, want the single artifact %q", i, p, paths[0])
		}
	}
}
