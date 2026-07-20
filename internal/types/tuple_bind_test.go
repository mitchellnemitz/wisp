package types

import "testing"

// pairFn is a helper prelude: a function returning (int, string).
const pairFn = "fn pair() -> (int,string) { return (1, \"a\") }\n"

// TestTupleBindBasic: `let (a: int, b: string) = pair()` binds a:int and
// b:string; using them type-checks.
func TestTupleBindBasic(t *testing.T) {
	expectOK(t, pairFn+"fn main() -> int {\nlet (a: int, b: string) = pair()\nlet x: int = a\nlet y: string = b\nreturn 0\n}")
	// final form binds the same way.
	expectOK(t, pairFn+"fn main() -> int {\nfinal (a: int, b: string) = pair()\nlet x: int = a\nlet y: string = b\nreturn 0\n}")
	// tuple literal RHS.
	expectOK(t, wrapMain("let (a: int, b: string) = (1, \"x\")\nlet u: int = a\nlet v: string = b"))
	// tuple-typed variable RHS.
	expectOK(t, wrapMain("let t: (int, string) = (1, \"x\")\nlet (a: int, b: string) = t\nlet u: int = a\nlet v: string = b"))
}

// TestTupleBindBareDiscard: a bare `_` slot imposes no type constraint and binds
// nothing; `out` binds. An annotated discard `_: int` checks the element type.
func TestTupleBindBareDiscard(t *testing.T) {
	expectOK(t, pairFn+"fn main() -> int {\nlet (_, out: string) = pair()\nlet y: string = out\nreturn 0\n}")
	// annotated discard with matching element type.
	expectOK(t, pairFn+"fn main() -> int {\nlet (_: int, out: string) = pair()\nlet y: string = out\nreturn 0\n}")
	// multiple `_` slots are fine.
	expectOK(t, pairFn+"fn main() -> int {\nlet (_, _) = pair()\nreturn 0\n}")
}

// TestTupleBindElemMismatchAtSlot: an element-type mismatch is a located error
// pointing at the OFFENDING slot, not the keyword.
func TestTupleBindElemMismatchAtSlot(t *testing.T) {
	// slot 0 declares string but element 0 is int -> error at slot 0.
	src := pairFn + "fn main() -> int {\nlet (a: string, b: string) = pair()\nreturn 0\n}"
	d := expectErr(t, src, "")
	// pairFn is line 1, `fn main` line 2, the `let (...)` line 3; slot 0 is the
	// `a` at column 6 (after "let ("). The error must point at the slot, not the
	// keyword's column 1.
	if d.Pos.Line != 3 {
		t.Fatalf("mismatch error at line %d, want slot line 3: %v", d.Pos.Line, d)
	}
	if d.Pos.Col != 6 {
		t.Fatalf("mismatch error at col %d, want slot col 6: %v", d.Pos.Col, d)
	}
	// annotated discard mismatch is also slot-located.
	expectErr(t, pairFn+"fn main() -> int {\nlet (_: string, b: string) = pair()\nreturn 0\n}", "")
}

// TestTupleBindAnnotatedDiscardSlotPos: the annotated-discard mismatch points at
// the discard slot's position.
func TestTupleBindAnnotatedDiscardSlotPos(t *testing.T) {
	src := pairFn + "fn main() -> int {\nlet (a: int, _: int) = pair()\nreturn 0\n}"
	d := expectErr(t, src, "")
	if d.Pos.Line != 3 {
		t.Fatalf("discard mismatch at line %d, want line 3: %v", d.Pos.Line, d)
	}
}

// TestTupleBindNonTupleRHS: a non-tuple RHS is a located error.
func TestTupleBindNonTupleRHS(t *testing.T) {
	d := expectErr(t, wrapMain("let (a: int, b: int) = 5"), "tuple")
	if d.Pos.Line == 0 {
		t.Fatalf("non-tuple error missing position: %v", d)
	}
}

// TestTupleBindWrongArity: an arity mismatch (RHS arity != slot count) is a
// located error.
func TestTupleBindWrongArity(t *testing.T) {
	// pair() is arity 2; pattern has 3 slots.
	d := expectErr(t, pairFn+"fn main() -> int {\nlet (a: int, b: string, c: int) = pair()\nreturn 0\n}", "arity")
	if d.Pos.Line == 0 {
		t.Fatalf("arity error missing position: %v", d)
	}
}

// TestTupleBindDuplicateName: a duplicate non-`_` name within one pattern is a
// located error at the offending (second) slot.
func TestTupleBindDuplicateName(t *testing.T) {
	src := "fn pair() -> (int,int) { return (1, 2) }\nfn main() -> int {\nlet (a: int, a: int) = pair()\nreturn 0\n}"
	d := expectErr(t, src, "")
	if d.Pos.Line != 3 {
		t.Fatalf("duplicate-name error at line %d, want slot line 3: %v", d.Pos.Line, d)
	}
}

// TestTupleBindFinalImmutable: a `final` destructured name is immutable;
// reassigning it is the same error as reassigning any final.
func TestTupleBindFinalImmutable(t *testing.T) {
	src := pairFn + "fn main() -> int {\nfinal (a: int, b: string) = pair()\na = 9\nreturn 0\n}"
	expectErr(t, src, "cannot assign to final")
	// `let` destructured names ARE mutable.
	expectOK(t, pairFn+"fn main() -> int {\nlet (a: int, b: string) = pair()\na = 9\nlet x: int = a\nlet y: string = b\nreturn 0\n}")
}

// TestTupleBindReadBlankErrors: reading `_` after an all-discard bind is still a
// compile error (the blank is not a binding).
func TestTupleBindReadBlankErrors(t *testing.T) {
	expectErr(t, pairFn+"fn main() -> int {\nlet (_, b: string) = pair()\nlet x: int = _\nlet y: string = b\nreturn 0\n}", "")
}

// TestTupleBindVarPosIsSlot: a bound name's Var.Pos is its SLOT position (the
// only path to slot-located LSP nav), NOT the keyword position. The Var lands in
// curFunc.Decls (the only path to codegen `local`).
func TestTupleBindVarPosIsSlot(t *testing.T) {
	src := pairFn + "fn main() -> int {\nlet (a: int, b: string) = pair()\nlet x: int = a\nlet y: string = b\nreturn 0\n}"
	info := expectOK(t, src)
	// Find the FuncInfo for main, then its Decls.
	var mainFI *FuncInfo
	for fn, fi := range info.Funcs {
		if fn.Name == "main" {
			mainFI = fi
		}
	}
	if mainFI == nil {
		t.Fatal("no FuncInfo for main")
	}
	var a, b *Var
	for _, v := range mainFI.Decls {
		switch v.Name {
		case "a":
			a = v
		case "b":
			b = v
		}
	}
	if a == nil || b == nil {
		t.Fatalf("destructured Vars a/b not appended to curFunc.Decls; Decls=%v", mainFI.Decls)
	}
	// pairFn is line 1, `fn main` line 2, the `let (...)` line 3. Slot positions
	// are on line 3; the columns differ from the keyword's column 1.
	if a.Pos.Line != 3 || b.Pos.Line != 3 {
		t.Fatalf("Var.Pos lines a=%d b=%d, want slot line 3", a.Pos.Line, b.Pos.Line)
	}
	if a.Pos.Col != 6 {
		t.Fatalf("a slot col=%d, want 6 (past `let (`)", a.Pos.Col)
	}
	if a.Pos.Col >= b.Pos.Col {
		t.Fatalf("slot columns not ordered: a.col=%d b.col=%d", a.Pos.Col, b.Pos.Col)
	}
	if a.Immutable || b.Immutable {
		t.Fatalf("let-bound names must be mutable: a.Immutable=%v b.Immutable=%v", a.Immutable, b.Immutable)
	}
	// final form sets Immutable.
	fsrc := pairFn + "fn main() -> int {\nfinal (a: int, b: string) = pair()\nlet x: int = a\nlet y: string = b\nreturn 0\n}"
	finfo := expectOK(t, fsrc)
	for fn, fi := range finfo.Funcs {
		if fn.Name != "main" {
			continue
		}
		for _, v := range fi.Decls {
			if (v.Name == "a" || v.Name == "b") && !v.Immutable {
				t.Fatalf("final-bound %q must be Immutable", v.Name)
			}
		}
	}
}

// TestTupleBindShadowing: a destructured name that shadows an enclosing binding
// is a located error at the SLOT position.
func TestTupleBindShadowing(t *testing.T) {
	src := pairFn + "fn main() -> int {\nlet a: int = 0\nlet (a: int, b: string) = pair()\nreturn 0\n}"
	d := expectErr(t, src, "shadowing")
	if d.Pos.Line != 4 {
		t.Fatalf("shadow error at line %d, want slot line 4: %v", d.Pos.Line, d)
	}
	if d.Pos.Col != 6 {
		t.Fatalf("shadow error at col %d, want slot col 6: %v", d.Pos.Col, d)
	}
}
