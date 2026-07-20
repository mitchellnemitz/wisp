package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// TestLinkedQualifiedGenericExplicitArgs: a generic function exported by another
// module can be called with explicit type arguments across the namespace.
func TestLinkedQualifiedGenericExplicitArgs(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.identity[int](5) }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn identity[T](x: T) -> T { return x }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

// TestLinkedQualifiedReturnOnlyExplicitArgs: a return-only type parameter on an
// exported generic is bindable only via explicit args, across modules.
func TestLinkedQualifiedReturnOnlyExplicitArgs(t *testing.T) {
	root := mod(t, 0,
		"fn main() -> int {\n  let xs: int[] = lib.empty[int]()\n  return length(xs)\n}",
		map[string]int{"lib": 1})
	lib := mod(t, 1, "export fn empty[T]() -> T[] {\n  let xs: T[] = []\n  return xs\n}", nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

// TestLinkedQualifiedCallerContextBinding: inside the caller's own generic [U], a
// qualified explicit arg `lib.identity[U](x)` binds to the CALLER's type parameter.
func TestLinkedQualifiedCallerContextBinding(t *testing.T) {
	root := mod(t, 0,
		"fn outer[U](x: U) -> U {\n  return lib.identity[U](x)\n}\n"+
			"fn main() -> int {\n  return outer[int](5)\n}",
		map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn identity[T](x: T) -> T { return x }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

// TestLinkedQualifiedCrossModuleStructArg: an explicit struct-typed arg resolves
// to the caller's struct identity and matches a same-struct value argument passed
// to the callee (guards resolve-before-swap: resolving in callee context would
// mint the wrong @modid token).
func TestLinkedQualifiedCrossModuleStructArg(t *testing.T) {
	root := mod(t, 0,
		"fn main() -> int {\n"+
			"  let v: a.Value = a.Value { n: 3 }\n"+
			"  return a.pick[a.Value](v)\n"+
			"}",
		map[string]int{"a": 1})
	a := mod(t, 1,
		"export struct Value { n: int }\n"+
			"export fn pick[T](x: T) -> int { return 0 }", nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, a}})
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
}

// TestLinkedQualifiedNonGeneric: type args on a non-generic exported function.
func TestLinkedQualifiedNonGeneric(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.plain[int](5) }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn plain(x: int) -> int { return x }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "is not generic") {
		t.Fatalf("want a not-generic error, got %v", errMsgs(info))
	}
}

// TestLinkedQualifiedMissingTargetWithTypeArgs: type args on a missing/unexported
// qualified target yield the existing single error (type args dropped on that path).
func TestLinkedQualifiedMissingTargetWithTypeArgs(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { return lib.nope[int](5) }`, map[string]int{"lib": 1})
	lib := mod(t, 1, `export fn other() -> int { return 1 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, lib}})
	if !hasErr(info, "has no function") {
		t.Fatalf("want a no-such-function error, got %v", errMsgs(info))
	}
}
