// internal/fuzz/shrink_test.go
package fuzz

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

func realTypeChecks(p *Program) bool {
	_, _, diags := driver.Compile("fuzz.wisp", Print(p))
	for _, d := range diags {
		if d.Severity == driver.Error {
			return false
		}
	}
	return true
}

func TestShrinkReducesButPreservesPredicate(t *testing.T) {
	keep := &LetStmt{Name: "keep", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &IntLit{V: "1"}, R: &IntLit{V: "2"}, T: Type{Kind: KInt}}}
	p := &Program{Body: []Stmt{
		&LetStmt{Name: "n1", T: Type{Kind: KInt}, Init: &IntLit{V: "5"}},
		keep,
		&LetStmt{Name: "n2", T: Type{Kind: KString}, Init: &StringLit{V: "junk"}},
		&PrintStmt{Arg: &Var{Name: "keep", T: Type{Kind: KInt}}},
	}}
	diverges := func(pr *Program) bool { return programMentionsVar(pr, "keep") }
	before := nodeCount(p)
	got := Shrink(p, realTypeChecks, diverges)
	if !diverges(got) {
		t.Fatal("shrink lost the divergence predicate")
	}
	if !realTypeChecks(got) {
		t.Fatalf("shrink produced ill-typed program:\n%s", Print(got))
	}
	if nodeCount(got) >= before {
		t.Fatalf("shrink did not reduce: before=%d after=%d", before, nodeCount(got))
	}
}

func TestShrinkIsLocallyMinimal(t *testing.T) {
	p := &Program{Body: []Stmt{
		&LetStmt{Name: "a", T: Type{Kind: KInt}, Init: &IntLit{V: "1"}},
		&LetStmt{Name: "keep", T: Type{Kind: KInt}, Init: &IntLit{V: "9"}},
		&PrintStmt{Arg: &Var{Name: "keep", T: Type{Kind: KInt}}},
	}}
	diverges := func(pr *Program) bool { return programMentionsVar(pr, "keep") }
	always := func(pr *Program) bool { return true }
	got := Shrink(p, always, diverges)
	for _, cand := range reductions(got) {
		if always(cand) && diverges(cand) {
			t.Fatal("not 1-minimal: a further reduction still satisfies both predicates")
		}
	}
}
