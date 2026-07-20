package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// TestMemberFuncref_EmitsWrapper: a namespaced-member funcref (`math.sqrt` in
// value position) lowers to the SAME __wisp_builtin_sqrt eta-expansion wrapper
// the bare-ident path uses, and tree-shakes in that wrapper plus its underlying
// helper. This is the Part 3 codegen path (Info.MemberFuncRefs).
func TestMemberFuncref_EmitsWrapper(t *testing.T) {
	out := genCore(t, `fn main() -> int {
  let f: fn(float) -> float = math.sqrt
  let _: float = f(4.0)
  return 0
}`, map[string]int{"math": 1}, "math")
	if !strings.Contains(out, "__wisp_builtin_sqrt") {
		t.Error("__wisp_builtin_sqrt wrapper not emitted for math.sqrt used as a value")
	}
	if !strings.Contains(out, "__wisp_sqrt") {
		t.Error("__wisp_sqrt helper not emitted (wrapper must dep its helper)")
	}
}

// TestMemberFuncref_SharesWrapperWithBare: referencing the same builtin as a
// value from two distinct namespaced sites (`math.sqrt` twice) mints one shared
// wrapper id, so the wrapper is defined exactly once (tree-shaken by id, not per
// reference site). PR C removed the bare `sqrt` value-reference surface, so the
// original bare-vs-namespaced sharing check is expressed here as namespaced-vs-
// namespaced; the wrapper-id sharing path is identical.
func TestMemberFuncref_SharesWrapperWithBare(t *testing.T) {
	out := genCore(t, `fn main() -> int {
  let a: fn(float) -> float = math.sqrt
  let b: fn(float) -> float = math.sqrt
  let _: float = a(1.0)
  let _: float = b(4.0)
  return 0
}`, map[string]int{"math": 1}, "math")
	if n := strings.Count(out, "__wisp_builtin_sqrt() {"); n != 1 {
		t.Errorf("wrapper defined %d times, want exactly 1 (two namespaced refs share one wrapper id)", n)
	}
}

// TestMemberFuncref_Runtime: a namespaced-member funcref (string.trim) actually
// runs correctly end to end when invoked indirectly through the wrapper.
func TestMemberFuncref_Runtime(t *testing.T) {
	root, err := parser.Parse(`fn main() -> int {
  let f: fn(string) -> string = string.trim
  print(f("  hi  "))
  return 0
}`, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: root, Namespaces: map[string]int{"string": 1}},
		{ID: 1, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: "string"},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	script, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	out, errb, code := run(t, script)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	if strings.TrimRight(out, "\n") != "hi" {
		t.Errorf("string.trim funcref output = %q, want %q", out, "hi")
	}
}

// TestMemberFuncref_UserFunctionCrossModule: a cross-module USER function
// (not a builtin) taken as a funcref value and invoked ONLY through that
// variable must survive tree-shaking (Fix #2) and the emitted call must
// actually run the target function (Fix #1).
func TestMemberFuncref_UserFunctionCrossModule(t *testing.T) {
	root, err := parser.Parse(`fn main() -> int {
  let h: fn(int) -> int = geo.double
  print(to_string(h(10)))
  return 0
}`, "test.wisp")
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	geo, err := parser.Parse(`export fn double(x: int) -> int { return x * 2 }`, "geo.wisp")
	if err != nil {
		t.Fatalf("parse geo: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: root, Namespaces: map[string]int{"geo": 1}},
		{ID: 1, Prog: geo, Namespaces: map[string]int{}},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	script, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	if !strings.Contains(string(script), "__wisp_f_m1_double() {") {
		t.Error("geo.double's definition was tree-shaken despite being referenced only as a funcref value")
	}
	out, errb, code := run(t, script)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	if strings.TrimRight(out, "\n") != "20" {
		t.Errorf("funcref call output = %q, want %q", out, "20")
	}
}
