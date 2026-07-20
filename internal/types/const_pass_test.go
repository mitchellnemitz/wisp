package types

// Task-3: const collect-fold pass + const-ref resolution.
//
// Tests cover:
//   - top-level const folded and usable in a function body
//   - const B referencing const A (declaration order)
//   - forward reference (X = Y before Y declared) rejected
//   - cyclic reference rejected
//   - type mismatch between folded value and annotation rejected

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// src wraps a const declaration block + a main function so the program is valid.
// body is placed inside fn main() -> int { ... return 0 }.
func constSrc(decls, body string) string {
	return decls + "\nfn main() -> int {\n" + body + "\nreturn 0\n}"
}

func TestConstPass_SimpleFold(t *testing.T) {
	// const A: int = 60*60 -- folds to 3600, no error
	src := constSrc("const A: int = 60*60", "")
	info := expectOK(t, src)
	// The const table should have A => 3600.
	if info.ConstTable == nil {
		t.Fatal("ConstTable is nil")
	}
	entry, ok := info.ConstTable["A"]
	if !ok {
		t.Fatal("const A not in ConstTable")
	}
	if entry.Value != int64(3600) {
		t.Fatalf("A folded value = %v, want 3600", entry.Value)
	}
	if entry.Type != Int {
		t.Fatalf("A type = %s, want int", entry.Type)
	}
}

func TestConstPass_RefResolution(t *testing.T) {
	// const A: int = 60*60
	// const B: int = A*2  -- B resolves A to 3600, folds to 7200
	src := constSrc("const A: int = 60*60\nconst B: int = A*2", "")
	info := expectOK(t, src)
	if info.ConstTable == nil {
		t.Fatal("ConstTable is nil")
	}
	b, ok := info.ConstTable["B"]
	if !ok {
		t.Fatal("const B not in ConstTable")
	}
	if b.Value != int64(7200) {
		t.Fatalf("B folded value = %v, want 7200", b.Value)
	}
}

func TestConstPass_ForwardRefRejected(t *testing.T) {
	// const X: int = Y  declared BEFORE Y -- forward reference is a compile error
	src := constSrc("const X: int = Y\nconst Y: int = 1", "")
	expectErr(t, src, "forward")
}

func TestConstPass_SelfRefRejected(t *testing.T) {
	// const A: int = A  -- self-reference is a compile error (cycle of length 1)
	src := constSrc("const A: int = A", "")
	expectErr(t, src, "cyclic")
}

func TestConstPass_CycleRejected(t *testing.T) {
	// const P: int = Q
	// const Q: int = P  -- mutual cycle; both consts must emit a cyclic error
	src := constSrc("const P: int = Q\nconst Q: int = P", "")
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected errors for mutual cycle, got none")
	}
	// Both P and Q must carry a "cyclic" label; neither should be mislabeled as
	// a plain "forward reference" (which would be accurate only for a genuine
	// one-directional forward ref, not a mutual cycle).
	for _, d := range info.Errors {
		if !strings.Contains(d.Msg, "cyclic") {
			t.Errorf("expected all cycle errors to contain %q, got: %s", "cyclic", d.Msg)
		}
	}
}

func TestConstPass_MinInt64Decl(t *testing.T) {
	// const MIN: int = -9223372036854775808 is accepted and folds to the min int64.
	src := constSrc("const MIN: int = -9223372036854775808", "")
	info := expectOK(t, src)
	entry, ok := info.ConstTable["MIN"]
	if !ok {
		t.Fatal("const MIN not in ConstTable")
	}
	if entry.Value != int64(-9223372036854775808) {
		t.Fatalf("MIN folded value = %v, want -9223372036854775808", entry.Value)
	}
}

func TestConstPass_AboveMaxDeclRejected(t *testing.T) {
	// const X: int = 9223372036854775808 (max+1, no sign) is still out of range.
	src := constSrc("const X: int = 9223372036854775808", "")
	expectErr(t, src, "out of range")
}

func TestConstPass_BelowMinDeclRejected(t *testing.T) {
	// const Y: int = -9223372036854775809 (one below min) is still out of range.
	src := constSrc("const Y: int = -9223372036854775809", "")
	expectErr(t, src, "out of range")
}

func TestConstPass_TypeMismatch(t *testing.T) {
	// const C: bool = 42  -- int folds to int but annotation says bool
	src := constSrc("const C: bool = 42", "")
	expectErr(t, src, "type mismatch")
}

func TestConstPass_ConstUsableAsDefaultArg(t *testing.T) {
	// const A: int = 3600 used as a default argument in main's parameter list,
	// verifying that the const is resolved through checkConstExpr during body
	// checking (not just recorded at collect-fold time).
	src := "const A: int = 3600\n" +
		"fn work(n: int = A) -> int { return n }\n" +
		"fn main() -> int { return work() }"
	info := expectOK(t, src)
	entry, ok := info.ConstTable["A"]
	if !ok {
		t.Fatal("const A not in ConstTable after body check")
	}
	if entry.Value != int64(3600) {
		t.Fatalf("A = %v, want 3600", entry.Value)
	}
}

func TestConstPass_TablePopulatedBeforeBodyCheck(t *testing.T) {
	// A function default arg references a top-level const. This exercises
	// the invariant that the collect-fold pass runs before Pass 4 (body check).
	// If the pass runs after bodies, checkConstExpr for the default arg would fail.
	src := "const TIMEOUT: int = 30\n" +
		"fn delay(n: int = TIMEOUT) -> int { return n }\n" +
		"fn main() -> int { return delay() }"
	expectOK(t, src)
}

// TestConstPass_DuplicateTopConstRejected verifies two top-level consts with the
// same name are a compile error (the second is rejected, the first is kept).
func TestConstPass_DuplicateTopConstRejected(t *testing.T) {
	src := constSrc("const X: int = 1\nconst X: int = 2", "")
	expectErr(t, src, "declared more than once")
}

// TestConstPass_ReservedTopConstRejected verifies a top-level const cannot reuse
// a reserved builtin name (e.g. print).
func TestConstPass_ReservedTopConstRejected(t *testing.T) {
	src := constSrc("const print: int = 1", "")
	expectErr(t, src, "reserved builtin or constant name")
}

// TestConstPass_ReservedNamespaceTopConstRejected verifies a top-level const
// cannot use the reserved "__" namespace.
func TestConstPass_ReservedNamespaceTopConstRejected(t *testing.T) {
	src := constSrc("const __x: int = 1", "")
	expectErr(t, src, "reserved \"__\" namespace")
}

// TestConstPass_TopConstFuncCollisionRejected verifies a top-level const name
// that collides with a top-level function is rejected.
func TestConstPass_TopConstFuncCollisionRejected(t *testing.T) {
	src := "const Foo: int = 1\n" +
		"fn Foo() -> int { return 0 }\n" +
		"fn main() -> int { return 0 }"
	expectErr(t, src, "declared function")
}

// TestConstPass_TopConstStructCollisionRejected verifies a top-level const name
// that collides with a top-level struct is rejected.
func TestConstPass_TopConstStructCollisionRejected(t *testing.T) {
	src := "const Foo: int = 1\n" +
		"struct Foo { x: int }\n" +
		"fn main() -> int { return 0 }"
	expectErr(t, src, "declared struct")
}

// TestConstPass_ValidTopConstsNoFalsePositive is the regression guard: distinct
// top-level consts, a top-level struct and func, plus a local const of a
// DIFFERENT name all type-check cleanly.
func TestConstPass_ValidTopConstsNoFalsePositive(t *testing.T) {
	src := "const A: int = 1\n" +
		"const B: int = 2\n" +
		"struct S { x: int }\n" +
		"fn helper() -> int { return A }\n" +
		"fn main() -> int {\n" +
		"const C: int = 3\n" +
		"return A + B + C + helper()\n" +
		"}"
	expectOK(t, src)
}

// TestConstPass_FailedTopConstReferencedNoPanic guards against a panic in the
// const resolver: a top-level const whose initializer fails to fold (10 / 0)
// is referenced from a later const during body checking, when foldingName is
// "". Before the fix the resolver dereferenced foldDeclIndex[""] (nil) and
// panicked; now it returns unresolved and the use site reports an error.
func TestConstPass_FailedTopConstReferencedNoPanic(t *testing.T) {
	src := constSrc("const A: int = 10 / 0", "const B: int = A + 1")
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected an error for the divide-by-zero const, got none")
	}
}

// TestConstPass_TopLevelConstRefRecordedInUses verifies that a const reference
// inside a TOP-LEVEL const initializer (folded with curFunc == nil) is recorded
// in info.Uses, so LSP find-references/rename reach it. A is referenced only in
// B's top-level initializer; before the fix the use was dropped.
func TestConstPass_TopLevelConstRefRecordedInUses(t *testing.T) {
	src := constSrc("const A: int = 1\nconst B: int = A + 1", "")
	info := expectOK(t, src)
	found := false
	for id, v := range info.Uses {
		if id.Name == "A" && v != nil && v.IsConst {
			found = true
		}
	}
	if !found {
		t.Fatal("reference to const A in a top-level const initializer was not recorded in info.Uses")
	}
}

// TestConstPass_LocalConstScopeLeak verifies a const declared in an inner block
// cannot be referenced from a const-expr outside that block: resolution is
// lexically scoped via the scope stack. Before the fix the flat localConsts map
// leaked X past its block and Y silently folded to 1.
func TestConstPass_LocalConstScopeLeak(t *testing.T) {
	src := `fn main() -> int {
  if (true) {
    const X: int = 1
  }
  const Y: int = X
  return Y
}`
	expectErr(t, src, "may not reference a variable")
}

// TestConstPass_BoolShortCircuit verifies && / || short-circuit during folding:
// an unreachable right operand is not evaluated, so a fault in it (divide by
// zero) is never reported, matching runtime short-circuit semantics.
func TestConstPass_BoolShortCircuit(t *testing.T) {
	// false && <RHS> folds to false; the divide-by-zero RHS is never evaluated.
	info := expectOK(t, constSrc("const A: bool = false && 10 / 0 == 0", ""))
	if a, ok := info.ConstTable["A"]; !ok {
		t.Fatal("const A not in ConstTable")
	} else if a.Value != false {
		t.Fatalf("A = %v, want false", a.Value)
	}
	// true || <RHS> folds to true; RHS never evaluated.
	info = expectOK(t, constSrc("const B: bool = true || 10 / 0 == 0", ""))
	if b, ok := info.ConstTable["B"]; !ok {
		t.Fatal("const B not in ConstTable")
	} else if b.Value != true {
		t.Fatalf("B = %v, want true", b.Value)
	}
}

// TestConstPass_BoolNoShortCircuitEvaluatesRHS verifies the non-short-circuiting
// path still evaluates the right operand: a fault in a REACHABLE RHS surfaces.
func TestConstPass_BoolNoShortCircuitEvaluatesRHS(t *testing.T) {
	// true && <erroring RHS>: RHS is reachable, so divide-by-zero is reported.
	expectErr(t, constSrc("const A: bool = true && 10 / 0 == 0", ""), "divide by zero")
}

// TestAssign_ReservedConstantRejected verifies assigning to a reserved constant
// (stdout/stderr) reports "cannot assign to constant" rather than falling
// through to "assignment to undeclared variable".
func TestAssign_ReservedConstantRejected(t *testing.T) {
	expectErr(t, constSrc("", "stdout = 5"), "cannot assign to constant")
}

// TestConstPass_BlankTopConstChecksAnnotation verifies a top-level blank const
// type-checks its annotation against the folded value (consistent with a
// function-local const _), even though it creates no binding.
func TestConstPass_BlankTopConstChecksAnnotation(t *testing.T) {
	expectErr(t, constSrc("const _: string = 42", ""), "type mismatch")
	expectOK(t, constSrc("const _: int = 42", ""))
}

// TestConstPass_UnresolvedAnnotationStaysInvalid verifies a top-level const whose
// type annotation fails to resolve does NOT fall back to the inferred folded
// type: its recorded type stays Invalid (consistent with let/final), so it
// cannot masquerade as a well-typed inferred const during later folding.
func TestConstPass_UnresolvedAnnotationStaysInvalid(t *testing.T) {
	src := constSrc("const X: NotAType = 5", "")
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected an unknown-type error for the unresolvable annotation")
	}
	if e, ok := info.ConstTable["X"]; ok && e.Type == Int {
		t.Errorf("const X with an unresolvable annotation must not infer type int; got %s", e.Type)
	}
}

func TestExportedConstRecordedAfterFold(t *testing.T) {
	prog, err := parser.Parse("export const OK: int = 1\nconst PRIV: int = 2", "m.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := &checker{info: newInfo()}
	c.modCtx = []*moduleCtx{{
		id: 0, prog: prog, namespaces: map[string]int{},
		funcs: map[string]*ast.FuncDecl{}, exported: map[string]bool{},
		structs:    map[string]*StructInfo{},
		constTable: map[string]*ConstEntry{}, topConsts: map[string]*Var{},
	}}
	c.cur = c.modCtx[0]
	c.collectFoldConsts(c.modCtx[0])
	if !c.modCtx[0].exported["OK"] {
		t.Errorf("exported const OK not recorded in exported set")
	}
	if c.modCtx[0].exported["PRIV"] {
		t.Errorf("non-exported const PRIV wrongly recorded as exported")
	}
}

func TestExportedConstFoldFailureNotRecorded(t *testing.T) {
	prog, err := parser.Parse("export const BAD: int = 1 / 0", "m.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := &checker{info: newInfo()}
	c.modCtx = []*moduleCtx{{
		id: 0, prog: prog, namespaces: map[string]int{},
		funcs: map[string]*ast.FuncDecl{}, exported: map[string]bool{},
		structs:    map[string]*StructInfo{},
		constTable: map[string]*ConstEntry{}, topConsts: map[string]*Var{},
	}}
	c.cur = c.modCtx[0]
	c.collectFoldConsts(c.modCtx[0])
	if c.modCtx[0].exported["BAD"] {
		t.Errorf("exported const that failed to fold was wrongly recorded as exported")
	}
}
