// internal/fuzz/donesignal_test.go   (NO build tag: runs in CI)
package fuzz

import "testing"

// intMinViaVar reports whether a program binds an INT_MIN source to a variable and
// then uses that variable as an arithmetic operand -- the exact path the documented
// revert (expr.go variable branch) breaks.
func intMinViaVar(p *Program) bool {
	imVars := map[string]bool{}
	for _, s := range p.Body {
		ls, ok := s.(*LetStmt)
		if !ok {
			continue
		}
		if binHasIntMinVarOperand(ls.Init, imVars) {
			return true
		}
		if isIntMinSource(ls.Init) {
			imVars[ls.Name] = true
		}
	}
	return false
}

func binHasIntMinVarOperand(e Expr, imVars map[string]bool) bool {
	b, ok := e.(*Binary)
	if !ok {
		return false
	}
	if isArithOp(b.Op) && (isIntMinVar(b.L, imVars) || isIntMinVar(b.R, imVars)) {
		return true
	}
	return binHasIntMinVarOperand(b.L, imVars) || binHasIntMinVarOperand(b.R, imVars)
}

func TestDoneSignalReachableViaVar(t *testing.T) {
	entries, err := LoadCanonicalManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		for i := 0; i < programCount(e); i++ {
			if intMinViaVar(generateAt(e.Seed, i, DefaultGenConfig())) {
				return
			}
		}
	}
	t.Fatal("no manifest program routes INT_MIN through a variable into arithmetic; the documented revert would produce no reproducer -- widen the manifest or bias the generator to bind INT_MIN then reuse it")
}
