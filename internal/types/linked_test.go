package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

func mod(t *testing.T, id int, src string, ns map[string]int) *module.Module {
	t.Helper()
	prog, err := parser.Parse(src, fmt.Sprintf("m%d.wisp", id))
	if err != nil {
		t.Fatalf("parse m%d: %v", id, err)
	}
	if ns == nil {
		ns = map[string]int{}
	}
	return &module.Module{ID: id, Prog: prog, Namespaces: ns}
}

// checkNS parses+checks src in a linked module set with each named core
// namespace bound (root at id 0, one synthetic core module per namespace).
// Modules-only analogue of check() for programs that reference namespaced
// members whose bare spelling was removed.
func checkNS(t *testing.T, src string, namespaces ...string) *Info {
	t.Helper()
	ns := map[string]int{}
	for i, n := range namespaces {
		ns[n] = i + 1
	}
	mods := []*module.Module{mod(t, 0, src, ns)}
	for i, n := range namespaces {
		mods = append(mods, coreMod(i+1, n))
	}
	return CheckLinked(&module.Linked{Modules: mods})
}

// expectOKNS is expectOK for a namespaced program.
func expectOKNS(t *testing.T, src string, namespaces ...string) *Info {
	t.Helper()
	info := checkNS(t, src, namespaces...)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s\nsrc:\n%s", diagList(info.Errors), src)
	}
	return info
}

// expectErrNS is expectErr for a namespaced program.
func expectErrNS(t *testing.T, src, want string, namespaces ...string) Diagnostic {
	t.Helper()
	info := checkNS(t, src, namespaces...)
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, want) {
			return d
		}
	}
	t.Fatalf("expected an error containing %q, got:\n%s\nsrc:\n%s", want, diagList(info.Errors), src)
	return Diagnostic{}
}

func errMsgs(info *Info) []string {
	out := make([]string, len(info.Errors))
	for i, e := range info.Errors {
		out[i] = e.Msg
	}
	return out
}

func hasErr(info *Info, substr string) bool {
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, substr) {
			return true
		}
	}
	return false
}

func TestLinkedCrossModuleCallAndExport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.fetch() }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn fetch() -> int { return 7 }`+"\n"+`fn priv() -> int { return 1 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
	// main is m0, fetch is m1.
	if info.Funcs[info.Main].Mangled != "__wisp_f_m0_main" {
		t.Errorf("main mangled = %q", info.Funcs[info.Main].Mangled)
	}
	var fetchMangled string
	for fn, fi := range info.Funcs {
		if fn.Name == "fetch" {
			fetchMangled = fi.Mangled
		}
	}
	if fetchMangled != "__wisp_f_m1_fetch" {
		t.Errorf("lib.fetch mangled = %q, want __wisp_f_m1_fetch", fetchMangled)
	}
}

func TestLinkedNonExportedCallErrors(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.priv() }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `fn priv() -> int { return 1 }`, nil) // not exported
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "not exported") {
		t.Fatalf("want a not-exported error, got %v", errMsgs(info))
	}
}

func TestLinkedDistinctStructsAcrossModules(t *testing.T) {
	// a.Value and b.Value are distinct types; passing one where the other is wanted
	// is a type error, and the message shows readable names (no '@').
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let x: a.Value = a.Value { n: 1 }`+"\n"+
			`  return b.take(x)`+"\n"+
			`}`,
		map[string]int{"a": 1, "b": 2})
	a := mod(t, 1, `export struct Value { n: int }`, nil)
	b := mod(t, 2, `export struct Value { n: int }`+"\n"+`export fn take(v: Value) -> int { return v.n }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, a, b}})
	if !hasErr(info, "want") {
		t.Fatalf("want a type-mismatch error for a.Value vs b.Value, got %v", errMsgs(info))
	}
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, "@") {
			t.Errorf("diagnostic leaked an internal token: %q", e.Msg)
		}
	}
}

func TestLinkedSameStructAcrossModulesOK(t *testing.T) {
	// Passing a.Value to a function that takes a.Value links cleanly.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let x: a.Value = a.Value { n: 5 }`+"\n"+
			`  return a.extract(x)`+"\n"+
			`}`,
		map[string]int{"a": 1})
	a := mod(t, 1, `export struct Value { n: int }`+"\n"+`export fn extract(v: Value) -> int { return v.n }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, a}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedCrossModuleGenericStructLetOK(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let b: bx.Box[int] = bx.Box { value: 42 }`+"\n"+
			`  return b.value`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Box[T] { value: T }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
	if _, ok := info.Structs["Box[int]@1"]; !ok {
		t.Fatalf("info.Structs missing Box[int]@1")
	}
}

func TestLinkedCrossModuleGenericStructInferenceOK(t *testing.T) {
	// This is the direct proof of the spec's central correctness claim:
	// a fixed unqualBase produces a token genericInstParts/isIdent can
	// parse, restoring Task #8's unify/applySubst/typeVarsIn
	// generic-struct case for cross-module tokens. unbox is called
	// with no explicit type argument, forcing inference of T from the
	// cross-module struct argument's shape.
	//
	// Deviation from the task-1 brief: the brief's snippet names this
	// function "unwrap", which collides with the builtin of the same
	// name (internal/types/builtins.go) and fails with an unrelated
	// "reserved builtin or constant name" error instead of exercising
	// the inference path. Renamed to "unbox" to isolate the intended
	// failure mode.
	root := mod(t, 0,
		`fn unbox[T](b: bx.Box[T]) -> T { return b.value }`+"\n"+
			`fn main() -> int {`+"\n"+
			`  return unbox(bx.Box { value: 42 })`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Box[T] { value: T }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedCrossModuleGenericStructParamOK(t *testing.T) {
	root := mod(t, 0,
		`fn get(b: bx.Box[int]) -> int { return b.value }`+"\n"+
			`fn main() -> int {`+"\n"+
			`  return get(bx.Box { value: 9 })`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Box[T] { value: T }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedCrossModuleGenericStructAliasOK(t *testing.T) {
	root := mod(t, 0,
		`type IB = bx.Box[int]`+"\n"+
			`fn main() -> int {`+"\n"+
			`  let b: IB = bx.Box { value: 7 }`+"\n"+
			`  return b.value`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Box[T] { value: T }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedCrossModuleGenericStructNested(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let b: bx.Box[bx.Box[int]] = bx.Box { value: bx.Box { value: 5 } }`+"\n"+
			`  return b.value.value`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Box[T] { value: T }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedCrossModuleGenericStructWrongArityErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let p: bx.Pair[int] = bx.Pair { first: 1, second: 2 }`+"\n"+
			`  return p.first`+"\n"+
			`}`,
		map[string]int{"bx": 1})
	bx := mod(t, 1, `export struct Pair[A, B] { first: A, second: B }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, bx}})
	if !hasErr(info, "requires 2 type argument(s), got 1") {
		t.Fatalf("want an arity error, got %v", errMsgs(info))
	}
}

func TestLinkedMainInNonRootErrors(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.f() }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn f() -> int { return 0 }`+"\n"+`fn main() -> int { return 1 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "only the root file may define") {
		t.Fatalf("want a non-root-main error, got %v", errMsgs(info))
	}
}

func TestLinkedNamespacePrecedence(t *testing.T) {
	// A local variable named like a namespace shadows it for field access; calling
	// through the namespace still works when no such local exists.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  return lib.fetch()`+"\n"+
			`}`,
		map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn fetch() -> int { return 3 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedNamespaceAsValueErrors(t *testing.T) {
	root := mod(t, 0, `fn main() -> int {`+"\n"+`let x: int = lib`+"\n"+`return x`+"\n"+`}`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn fetch() -> int { return 1 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "module namespace, not a value") {
		t.Fatalf("want namespace-as-value error, got %v", errMsgs(info))
	}
}

func TestLinkedConstDoesNotLeakAcrossModules(t *testing.T) {
	// A top-level const declared only in lib (m1) must NOT be resolvable by bare
	// name from the root (m0). const is file-local (no `export const` yet), so a
	// bare reference to SECRET in m0 is an undeclared name, not m1's value.
	root := mod(t, 0, `fn main() -> int { return SECRET }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `const SECRET: int = 42`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "undeclared name") {
		t.Fatalf("want an undeclared-name error for cross-module const leak, got %v", errMsgs(info))
	}
}

func TestLinkedSameNamedConstsDoNotCollide(t *testing.T) {
	// Two modules each declare a file-local const X of a different type. Each
	// module must resolve its OWN X; a shared bare-name table would overwrite one
	// with the other (last-wins) and spuriously type-check m0's int X as string.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let v: int = X`+"\n"+
			`  return v`+"\n"+
			`}`+"\n"+
			`const X: int = 1`,
		map[string]int{"lib": 1})
	lib := mod(t, 1, `const X: string = "two"`+"\n"+`export fn f() -> string { return X }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("same-named file-local consts must not collide across modules, got %v", errMsgs(info))
	}
}

func TestLinkedUnexportedTypeErrors(t *testing.T) {
	root := mod(t, 0, `fn main() -> int {`+"\n"+`let x: lib.Hidden = lib.Hidden { n: 1 }`+"\n"+`return x.n`+"\n"+`}`, map[string]int{"lib": 1})
	lib := mod(t, 1, `struct Hidden { n: int }`, nil) // not exported
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "not exported") {
		t.Fatalf("want not-exported type error, got %v", errMsgs(info))
	}
}

func TestLinkedExportedConstResolves(t *testing.T) {
	// int, string, bool, and float exported consts all resolve in a body (AC2).
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = util.MAX`+"\n"+
			`  let g: string = util.GREETING`+"\n"+
			`  let f: bool = util.FLAG`+"\n"+
			`  let r: float = util.RATIO`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"util": 1})
	util := mod(t, 1,
		`export const MAX: int = 3`+"\n"+
			`export const GREETING: string = "hi"`+"\n"+
			`export const FLAG: bool = true`+"\n"+
			`export const RATIO: float = 1.5`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedExportedConstTypeMismatchErrors(t *testing.T) {
	// util.GREETING is a string; assigning to an int binding is a type error,
	// proving the resolved TYPE flows from the producing module's ConstEntry.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = util.GREETING`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export const GREETING: string = "hi"`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if len(info.Errors) == 0 {
		t.Fatalf("expected a type-mismatch error")
	}
}

func TestLinkedNonExportedConstErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int { return util.PRIVATE }`,
		map[string]int{"util": 1})
	util := mod(t, 1, `const PRIVATE: int = 5`, nil) // not exported
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if !hasErr(info, "is not exported from") {
		t.Fatalf("want a not-exported error, got %v", errMsgs(info))
	}
}

func TestLinkedNoSuchConstErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int { return util.NOPE }`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export const MAX: int = 3`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if !hasErr(info, "has no exported constant") {
		t.Fatalf("want a no-exported-constant error, got %v", errMsgs(info))
	}
}

func TestLinkedReExportRejected(t *testing.T) {
	// mid imports base.X but does not `export const` it; root.mid.X must NOT
	// silently resolve to base's value -- it is a no-exported-constant error.
	root := mod(t, 0,
		`fn main() -> int { return mid.X }`,
		map[string]int{"mid": 1})
	mid := mod(t, 1, `fn use_it() -> int { return base.X }`, map[string]int{"base": 2})
	base := mod(t, 2, `export const X: int = 9`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, mid, base}})
	if !hasErr(info, "has no exported constant") {
		t.Fatalf("want re-export rejected as no-exported-constant, got %v", errMsgs(info))
	}
}

func TestLinkedExportedFnViaNsInValuePositionNotConst(t *testing.T) {
	// util.fetch is an exported FUNCTION; using it as a value via ns.NAME now
	// mints a FuncRef (this fix's whole point) -- AC16's "fn as a bare value is
	// never a const" rule is superseded for this GENERAL value-position case.
	// AC16 still holds for the const-initializer/default-argument context; see
	// TestQualifiedUserFuncAsDefaultArgumentStillErrors for that boundary. Here,
	// the annotation `int` doesn't match the minted funcref type `fn()->int`, so
	// the failure that remains is an ordinary type mismatch, not a wrong-kind
	// diagnostic.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = util.fetch`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export fn fetch() -> int { return 1 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if !hasErr(info, "want") {
		t.Fatalf("want a type-mismatch error, got %v", errMsgs(info))
	}
	// It must not be mistaken for an exported/non-exported const.
	if hasErr(info, "is not exported from") || hasErr(info, "has no exported constant") {
		t.Errorf("fn-as-value wrongly produced a const diagnostic: %v", errMsgs(info))
	}
}

func TestLinkedExportedStructViaNsInValuePositionNotConst(t *testing.T) {
	// util.Point is an exported STRUCT; using it as a bare value via ns.NAME is
	// NOT a const (AC16, struct parallel to the fn case).
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = util.Point`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export struct Point { x: int, y: int }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if !hasErr(info, "is a struct of") || !hasErr(info, "not a constant") {
		t.Fatalf("want a wrong-kind (struct, not a constant) error, got %v", errMsgs(info))
	}
	if hasErr(info, "is not exported from") || hasErr(info, "has no exported constant") {
		t.Errorf("struct-as-value wrongly produced a const diagnostic: %v", errMsgs(info))
	}
}

func TestLinkedNamespaceShadowedByLocalNotResolvedAsConst(t *testing.T) {
	// R3 shadowing: a parameter named like the namespace alias (util) shadows it,
	// so `util.MAX` is NOT a cross-module const reference -- qualifiedNsTarget
	// returns ok == false (c.lookup("util") != nil) and the FieldAccess falls
	// through to ordinary struct-field handling, which errors because the local
	// `util` is an int, not a struct. The cross-module const value (3) must NOT be
	// resolved here: there must be no const diagnostic and no successful resolution.
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  return shadow(7)`+"\n"+
			`}`+"\n"+
			`fn shadow(util: int) -> int {`+"\n"+
			`  return util.MAX`+"\n"+
			`}`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export const MAX: int = 3`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if len(info.Errors) == 0 {
		t.Fatalf("expected a field-access error on the shadowing local, got none")
	}
	// The shadowed alias must NOT have been resolved as a cross-module const: no
	// const-resolution diagnostic, and the field-access path (non-struct base)
	// reports instead.
	if hasErr(info, "is not exported from") || hasErr(info, "has no exported constant") ||
		hasErr(info, "not a constant") {
		t.Errorf("shadowed alias wrongly entered the qualified-const path: %v", errMsgs(info))
	}
	if !hasErr(info, "cannot access field") {
		t.Errorf("want the ordinary field-access diagnostic on a non-struct local, got %v", errMsgs(info))
	}
}

func TestLinkedExportedConstInDefaultArg(t *testing.T) {
	// AC3/R10: ns.NAME resolves in a default-argument expression (foldAllowsQualified).
	root := mod(t, 0,
		`fn pause(secs: int = util.TIMEOUT) -> int { return secs }`+"\n"+
			`fn main() -> int { return pause() }`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export const TIMEOUT: int = 30`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

func TestLinkedConstInitCrossModuleRejected(t *testing.T) {
	// AC6: a const initializer is file-local -- util.MAX inside a const-expr is an
	// error even though the SAME shape is accepted in a default arg above.
	root := mod(t, 0,
		`const DERIVED: int = util.MAX`+"\n"+
			`fn main() -> int { return DERIVED }`,
		map[string]int{"util": 1})
	util := mod(t, 1, `export const MAX: int = 3`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, util}})
	if len(info.Errors) == 0 {
		t.Fatalf("expected an error for a cross-module const initializer")
	}
}

func TestLinkedExportedConstOrderInsensitive(t *testing.T) {
	// Ordering A: root(0) imports producer(1). Producer declared AFTER importer in
	// the modules slice relative to a sibling importer.
	rootA := mod(t, 0, `fn main() -> int { return util.MAX + lib.also() }`,
		map[string]int{"util": 1, "lib": 2})
	prodA := mod(t, 1, `export const MAX: int = 3`, nil)
	libA := mod(t, 2, `export fn also() -> int { return util2.MAX }`, map[string]int{"util2": 1})
	infoA := CheckLinked(&module.Linked{Modules: []*module.Module{rootA, prodA, libA}})
	if len(infoA.Errors) != 0 {
		t.Fatalf("ordering A errors: %v", errMsgs(infoA))
	}

	// Ordering B: the importer module (lib, modid 1) appears in the slice BEFORE
	// the producer module (prod, modid 2). lib reaches prod via its own namespace.
	rootB := mod(t, 0, `fn main() -> int { return lib.also() }`, map[string]int{"lib": 1})
	libB := mod(t, 1, `export fn also() -> int { return prod.MAX }`, map[string]int{"prod": 2})
	prodB := mod(t, 2, `export const MAX: int = 3`, nil)
	infoB := CheckLinked(&module.Linked{Modules: []*module.Module{rootB, libB, prodB}})
	if len(infoB.Errors) != 0 {
		t.Fatalf("ordering B errors: %v", errMsgs(infoB))
	}
}

func TestQualifiedUserFuncAsFuncrefValueOK(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let h: fn(int) -> int = geo.double`+"\n"+
			`  return h(1)`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export fn double(x: int) -> int { return x * 2 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", errMsgs(info))
	}
}

func TestQualifiedUserFuncAsFuncrefValueUnexportedErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let h: fn(int) -> int = geo.double`+"\n"+
			`  return h(1)`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `fn double(x: int) -> int { return x * 2 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, `"double" is not exported by "geo"`) {
		t.Fatalf("want an unexported-function error, got %v", errMsgs(info))
	}
}

func TestQualifiedUserFuncAsFuncrefValueGenericErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let h: fn(int) -> int = geo.identity`+"\n"+
			`  return h(1)`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export fn identity[T](x: T) -> T { return x }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, `generic function "identity" cannot be used as a function reference`) {
		t.Fatalf("want a generic-function-as-funcref error, got %v", errMsgs(info))
	}
}

func TestQualifiedUserFuncAsFuncrefValueSignatureMismatchErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let h: fn(int) -> int = geo.double`+"\n"+
			`  return h(1)`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export fn double(x: string) -> string { return x }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, "fn(string)->string") {
		t.Fatalf("want a type-mismatch error naming the real funcref type fn(string)->string, got %v", errMsgs(info))
	}
}

func TestQualifiedUnresolvableFieldStillErrors(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = geo.nonexistent`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export fn double(x: int) -> int { return x * 2 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, `has no exported constant "nonexistent"`) {
		t.Fatalf("want the unresolvable-field fall-through error, got %v", errMsgs(info))
	}
}

func TestQualifiedStructNameStillErrorsAsNotAConstant(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let n: int = geo.Point`+"\n"+
			`  return n`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export struct Point { x: int, y: int }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, "is a struct of") {
		t.Fatalf("want the struct-as-value error, got %v", errMsgs(info))
	}
}

func TestQualifiedUserFuncAsFuncrefValueStructParamOK(t *testing.T) {
	root := mod(t, 0,
		`fn main() -> int {`+"\n"+
			`  let h: fn(int) -> geo.Point = geo.make`+"\n"+
			`  let p: geo.Point = h(3)`+"\n"+
			`  return p.x`+"\n"+
			`}`,
		map[string]int{"geo": 1})
	geo := mod(t, 1,
		`export struct Point { x: int, y: int }`+"\n"+
			`export fn make(x: int) -> Point { return Point { x: x, y: 0 } }`,
		nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors (context swap required for cross-module struct param/return), got %v", errMsgs(info))
	}
}

func TestQualifiedUserFuncAsDefaultArgumentStillErrors(t *testing.T) {
	root := mod(t, 0,
		`fn f(cb: fn(int) -> int = geo.double) -> int { return cb(1) }`+"\n"+
			`fn main() -> int { return f() }`,
		map[string]int{"geo": 1})
	geo := mod(t, 1, `export fn double(x: int) -> int { return x * 2 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, geo}})
	if !hasErr(info, "not a constant expression") {
		t.Fatalf("want a not-a-constant-expression error, got %v", errMsgs(info))
	}
}
