package golden

import (
	"bytes"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// TestTreeShaking asserts the prelude is tree-shaken (spec section 9.1 / AC 5):
// a minimal program that uses only print must NOT emit unused helpers such as
// __wisp_replace.
func TestTreeShaking(t *testing.T) {
	src := "fn main() -> int {\n  print(\"hi\")\n  return 0\n}\n"
	script, _, diags := driver.Compile("tiny.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_replace", "__wisp_trim", "__wisp_lower", "__wisp_upper", "__wisp_bool"} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: unused helper %q present in minimal program output", helper)
		}
	}
	// Sanity: the program does use print, so its helper must be present.
	if !bytes.Contains(script, []byte("print()")) {
		t.Errorf("expected the print helper to be present")
	}
	// M3: a program that uses no aggregate must not emit the handle runtime.
	for _, helper := range []string{"__wisp_alloc", "__wisp_bounds_fail", "__wisp_next_id"} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: aggregate helper %q present in a program using no aggregate", helper)
		}
	}
	// M3 PR-C: a program that uses no dict must not emit the dict runtime.
	for _, helper := range []string{"__wisp_dkey_enc", "__wisp_dkey_dec", "__wisp_hexdig", "__wisp_dict_miss"} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: dict helper %q present in a program using no dict", helper)
		}
	}
	// M5: a program with no try/throw emits NO error-handling scaffolding (the
	// zero-overhead requirement, spec invariant 14).
	for _, scaffold := range []string{"__wisp_try_depth", "__wisp_err_pending", "__wisp_err_msg", "__wisp_throw"} {
		if bytes.Contains(script, []byte(scaffold)) {
			t.Errorf("zero-overhead failed: %q present in a program with no try/throw", scaffold)
		}
	}
	// M6: a program that uses none of the core stdlib builtins must not emit their
	// helpers.
	for _, helper := range []string{
		"__wisp_split", "__wisp_join", "__wisp_contains", "__wisp_starts_with",
		"__wisp_ends_with", "__wisp_index_of", "__wisp_repeat", "__wisp_fabs",
	} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: unused M6 helper %q present in a minimal program", helper)
		}
	}
}

// TestM6StdlibHelpersEmittedWhenUsed asserts the converse: a program using the
// M6 string/array stdlib emits exactly the helpers it needs.
func TestM6StdlibHelpersEmittedWhenUsed(t *testing.T) {
	src := `import "string"
fn main() -> int {
  let parts: string[] = string.split("a,b", ",")
  print(string.join(parts, "-"))
  let c: bool = string.contains("ab", "a")
  let i: int = unwrap_or(string.index_of("ab", "b"), -1)
  print(string.repeat("x", 2))
  return 0
}
`
	script, _, diags := driver.Compile("m6.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_split", "__wisp_join", "__wisp_contains", "__wisp_index_of", "__wisp_repeat"} {
		if !bytes.Contains(script, []byte(helper)) {
			t.Errorf("expected M6 helper %q to be present when used", helper)
		}
	}
	// starts_with/ends_with/fabs are NOT used here -> must be absent.
	for _, helper := range []string{"__wisp_starts_with", "__wisp_ends_with", "__wisp_fabs"} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: unused helper %q present", helper)
		}
	}
}

// TestErrorScaffoldingEmittedWhenUsed asserts the converse: a try/throw program
// emits the runtime state init, the mode-aware fail, and __wisp_throw.
func TestErrorScaffoldingEmittedWhenUsed(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  try { throw error(\"x\") } catch (e) { print(e.message) }\n" +
		"  return 0\n}\n"
	script, _, diags := driver.Compile("err.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, scaffold := range []string{"__wisp_try_depth=0", "__wisp_throw", "__wisp_err_pending"} {
		if !bytes.Contains(script, []byte(scaffold)) {
			t.Errorf("expected error scaffolding %q to be present when try/throw is used", scaffold)
		}
	}
}

// TestDictHelpersEmittedWhenUsed asserts the converse: a dict-using program
// emits the encode/decode helpers and (for a lookup) the missing-key abort.
func TestDictHelpersEmittedWhenUsed(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let m: {string: int} = { \"a\": 1 }\n" +
		"  print(to_string(m[\"a\"]))\n" +
		"  for (k in m) { print(k) }\n" +
		"  return 0\n}\n"
	script, _, diags := driver.Compile("dict.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_dkey_enc", "__wisp_dkey_dec", "__wisp_dict_miss", "__wisp_alloc"} {
		if !bytes.Contains(script, []byte(helper)) {
			t.Errorf("expected dict helper %q to be present when a dict is used", helper)
		}
	}
}

// TestAggregateHelpersEmittedWhenUsed asserts the converse: a program that uses
// an array emits the alloc helper, and an indexing program emits the bounds
// helper (M3 PR-B). A struct-only program needs alloc but not bounds.
func TestAggregateHelpersEmittedWhenUsed(t *testing.T) {
	src := "fn main() -> int {\n  let xs: int[] = [1, 2]\n  print(to_string(xs[0]))\n  return 0\n}\n"
	script, _, diags := driver.Compile("agg.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_alloc", "__wisp_bounds_fail"} {
		if !bytes.Contains(script, []byte(helper)) {
			t.Errorf("expected aggregate helper %q to be present when arrays/indexing are used", helper)
		}
	}
}

// TestRemovedSugarHelpersAbsent is the SC-015 gate: a program exercising the
// unwrap_or(parse_int/parse_float/dict.get/env.get, fb) replacement path for
// the removed int_or/float_or/get_or/env_or sugar must emit none of their old
// runtime helpers. get_or had no runtime helper of its own (fully inlined
// codegen), so its removal is covered by go build/test, not a grep here.
func TestRemovedSugarHelpersAbsent(t *testing.T) {
	src := `import "env"
fn main() -> int {
  print(to_string(unwrap_or(parse_int("42"), -1)))
  print(to_string(unwrap_or(parse_float("3.14"), -1.0)))
  print(unwrap_or(env.get("PATH"), "FB"))
  return 0
}
`
	script, _, diags := driver.Compile("sugar.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_int_or", "__wisp_float_or", "__wisp_env_or"} {
		if bytes.Contains(script, []byte(helper)) {
			t.Errorf("tree-shaking failed: removed helper %q present in emitted output", helper)
		}
	}
}

// TestJSONTreeShaking asserts the json engine and wrappers are tree-shaken: a
// program with no `import "json"` emits none of them, and the tree-shaking is
// selective (json.encode/from_int/null do NOT pull the engine; from_string
// does, for the escape pass).
func TestJSONTreeShaking(t *testing.T) {
	// No json use at all.
	plain, _, diags := driver.Compile("p.wisp", "fn main() -> int {\n  print(\"hi\")\n  return 0\n}\n")
	if errored(diags) {
		t.Fatalf("compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_json_awk", "__wisp_json_validate", "__wisp_json_escape", "__wisp_json_get", "__wisp_j_"} {
		if bytes.Contains(plain, []byte(helper)) {
			t.Errorf("tree-shaking failed: json helper %q present in a non-json program", helper)
		}
	}

	// encode/from_int/null must NOT pull the awk engine (no scan needed).
	noEngine, _, diags := driver.Compile("e.wisp", "import \"json\"\nfn main() -> int {\n  print(json.encode(json.from_int(1)))\n  return 0\n}\n")
	if errored(diags) {
		t.Fatalf("compile errors: %v", diags)
	}
	if bytes.Contains(noEngine, []byte("__wisp_json_awk")) {
		t.Errorf("tree-shaking failed: json.encode(from_int) must not pull the awk engine")
	}

	// decode DOES pull the engine + validate.
	withEngine, _, diags := driver.Compile("d.wisp", "import \"json\"\nfn main() -> int {\n  print(json.encode(json.decode(\"1\")))\n  return 0\n}\n")
	if errored(diags) {
		t.Fatalf("compile errors: %v", diags)
	}
	for _, helper := range []string{"__wisp_json_awk", "__wisp_json_validate"} {
		if !bytes.Contains(withEngine, []byte(helper)) {
			t.Errorf("expected %q present in a program that calls json.decode", helper)
		}
	}
}
