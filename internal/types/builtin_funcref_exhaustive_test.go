package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// TestBuiltinFuncref_Exhaustive drives the checker over EVERY builtin in
// builtinSigs and asserts the universal-funcref invariant end to end:
//
//   - a generatable builtin, referenced in value position with its derived
//     funcref type, type-checks with no errors and records exactly one FuncRef
//     whose Mangled is the __wisp_builtin_<name> wrapper; and
//   - every other builtin is REJECTED in value position (at least one error) and
//     records NO FuncRef.
//
// It also cross-checks BuiltinFuncrefClassOf against membership: generatable
// names land in one of the three generatable classes, the rest in a rejected
// class. This is the single exhaustive gate that no builtin is silently
// mishandled -- a new builtin added to builtinSigs is forced into exactly one
// side of this split. Wrapper-existence (the runtime half) is asserted by the
// cross-package consistency test in internal/codegen, since types must not
// import runtime.
func TestBuiltinFuncref_Exhaustive(t *testing.T) {
	generatableClasses := map[BuiltinFuncrefClass]bool{
		FuncrefMonomorphic: true, FuncrefVoid: true, FuncrefNullary: true,
	}
	for name := range builtinSigs {
		// Removable builtins have no bare surface anymore: a value-position
		// reference yields the moved-to-module diagnostic, not a funcref. Their
		// funcref behavior lives in the namespaced core_*_test.go suites. Restrict
		// this exhaustive gate to the stays-flat set.
		if isRemovableBuiltin(name) {
			continue
		}
		gen := builtinFuncrefGeneratable[name]
		class := BuiltinFuncrefClassOf(name)
		if gen != generatableClasses[class] {
			t.Errorf("%s: generatable=%v but class=%q (class/membership disagree)", name, gen, class)
		}

		if gen {
			ft := string(builtinFuncrefType(name))
			src := "fn main() -> int {\n\tlet f: " + ft + " = " + name + "\n\treturn 0\n}"
			prog, err := parser.Parse(src, "test.wisp")
			if err != nil {
				t.Errorf("%s: generatable builtin failed to parse in value position: %v", name, err)
				continue
			}
			info := Check(prog)
			if len(info.Errors) != 0 {
				t.Errorf("%s: generatable but value-position reference errored: %s", name, diagList(info.Errors))
				continue
			}
			if len(info.FuncRefs) != 1 {
				t.Errorf("%s: expected exactly 1 FuncRef recorded, got %d", name, len(info.FuncRefs))
				continue
			}
			want := builtinFuncrefMangled(name)
			for _, fr := range info.FuncRefs {
				if fr.Mangled != want {
					t.Errorf("%s: FuncRef.Mangled = %q, want %q", name, fr.Mangled, want)
				}
				if fr.Type != Type(ft) {
					t.Errorf("%s: FuncRef.Type = %q, want %q", name, fr.Type, ft)
				}
			}
			continue
		}

		// Non-generatable: value position must be rejected. Some builtin names are
		// also type keywords (e.g. "error"), which the PARSER rejects as a bare
		// value ident before the checker runs -- that is still a valid rejection.
		// The annotation is a deliberately-simple dummy; the rejection fires on the
		// builtin reference regardless of the annotation.
		src := "fn main() -> int {\n\tlet f: fn()->int = " + name + "\n\treturn 0\n}"
		prog, err := parser.Parse(src, "test.wisp")
		if err != nil {
			// Parser-level rejection (type-keyword collision): correct outcome.
			continue
		}
		info := Check(prog)
		if len(info.Errors) == 0 {
			t.Errorf("%s: non-generatable builtin accepted in value position (want a rejection)", name)
		}
		if len(info.FuncRefs) != 0 {
			t.Errorf("%s: non-generatable builtin recorded a FuncRef (want none)", name)
		}
	}
}
