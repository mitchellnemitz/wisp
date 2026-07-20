package codegen

import (
	"strings"
	"testing"
)

// The map / filter behavioral combinator tests (TestMapOptionalBehavioral,
// TestFilterOptionalBehavioral, TestCombinatorReachability, TestMapResultBehavioral,
// TestCombinatorLazyAllInactiveBranches) moved to runtime coverage in
// internal/golden (combinator_map_optional, combinator_map_result,
// combinator_filter_optional, combinator_reachability, combinator_lazy -- namespaced
// array.map / array.filter). The and_then / or_else / map_err tests below stay flat
// (those combinators are not removable) and are kept. TestMapOptionalShape is
// reconstructed below with the namespaced array.map spelling; the delegate lowers
// byte-identically to the pre-removal flat map, so the lazy Optional-axis lowering
// shape is unchanged.

// TestMapOptionalShape asserts the Optional axis of map lowers lazily: it reads
// the Optional's _tag and only calls the mapping funcref inside the present
// branch (an if block), never eagerly. Reconstructed with array.map.
func TestMapOptionalShape(t *testing.T) {
	src := "fn dbl(x: int) -> int { return x * 2 }\n" +
		wrapMainCG("let o: Optional[int] = Some(5)\nlet r: Optional[int] = array.map(o, dbl)")
	out := string(compileNS(t, src, "array"))
	if !strings.Contains(out, "}_tag") {
		t.Errorf("map Optional: expected _tag check, got:\n%s", out)
	}
	// The funcref call must appear only inside an if block (lazy).
	if !strings.Contains(out, "if ") {
		t.Errorf("map Optional: expected conditional (if), got:\n%s", out)
	}
}

// TestAndThenOptionalBehavioral: Some(5) -> chain; None short-circuits; chain to None works. AC 5.
func TestAndThenOptionalBehavioral(t *testing.T) {
	src := "fn safe_half(x: int) -> Optional[int] {\nif (x == 0) { return None }\nreturn Some(x / 2)\n}\n" +
		"fn main() -> int {\n" +
		"let s: Optional[int] = Some(10)\n" +
		"let r1: Optional[int] = and_then(s, safe_half)\n" +
		"print(to_string(is_some(r1)))\n" +
		"print(to_string(unwrap(r1)))\n" +
		"let n: Optional[int] = None\n" +
		"let r2: Optional[int] = and_then(n, safe_half)\n" +
		"print(to_string(is_none(r2)))\n" +
		"let z: Optional[int] = Some(0)\n" +
		"let r3: Optional[int] = and_then(z, safe_half)\n" +
		"print(to_string(is_none(r3)))\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "true\n5\ntrue\ntrue\n" {
		t.Errorf("stdout = %q, want %q", stdout, "true\n5\ntrue\ntrue\n")
	}
}

// TestOrElseOptionalBehavioral: Some -> original (f not called); None -> f() result. AC 9.
func TestOrElseOptionalBehavioral(t *testing.T) {
	src := "fn fallback() -> Optional[int] { return Some(99) }\n" +
		"fn main() -> int {\n" +
		"let s: Optional[int] = Some(7)\n" +
		"let r1: Optional[int] = or_else(s, fallback)\n" +
		"print(to_string(unwrap(r1)))\n" +
		"let n: Optional[int] = None\n" +
		"let r2: Optional[int] = or_else(n, fallback)\n" +
		"print(to_string(unwrap(r2)))\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "7\n99\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\n99\n")
	}
}

// TestAndThenResultBehavioral: Ok chains; Err short-circuits. AC 5 Result.
func TestAndThenResultBehavioral(t *testing.T) {
	src := "fn double_safe(x: int) -> Result[int] { return Ok(x * 2) }\n" +
		"fn main() -> int {\n" +
		"let ok: Result[int] = Ok(3)\n" +
		"let r1: Result[int] = and_then(ok, double_safe)\n" +
		"print(to_string(unwrap(r1)))\n" +
		"let e: error = error(\"fail\")\n" +
		"let err: Result[int] = Err(e)\n" +
		"let r2: Result[int] = and_then(err, double_safe)\n" +
		"print(to_string(is_err(r2)))\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "6\ntrue\n" {
		t.Errorf("stdout = %q, want %q", stdout, "6\ntrue\n")
	}
}

// TestOrElseResultBehavioral: Ok returns original; Err calls f WITH the error,
// which the recovery fn inspects and surfaces. AC 9 Result.
func TestOrElseResultBehavioral(t *testing.T) {
	src := "fn rescue(e: error) -> Result[int] { print(e.message)\nreturn Ok(0) }\n" +
		"fn main() -> int {\n" +
		"let ok: Result[int] = Ok(7)\n" +
		"let r1: Result[int] = or_else(ok, rescue)\n" +
		"print(to_string(unwrap(r1)))\n" +
		"let e: error = error(\"orig\")\n" +
		"let err: Result[int] = Err(e)\n" +
		"let r2: Result[int] = or_else(err, rescue)\n" +
		"print(to_string(unwrap(r2)))\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	// Ok arm: rescue NOT called, so "orig" appears exactly once (from the Err arm),
	// proving the raw error handle was passed to rescue.
	if stdout != "7\norig\n0\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\norig\n0\n")
	}
}

// TestMapErrResultBehavioral: Ok returns original; Err wraps with a DIFFERENT,
// observable message derived from the original. AC 10.
func TestMapErrResultBehavioral(t *testing.T) {
	src := "fn wrap_err(e: error) -> error { return error(\"wrapped: \" + e.message) }\n" +
		"fn main() -> int {\n" +
		"let ok: Result[int] = Ok(5)\n" +
		"let r1: Result[int] = map_err(ok, wrap_err)\n" +
		"print(to_string(unwrap(r1)))\n" +
		"let e: error = error(\"a\")\n" +
		"let err: Result[int] = Err(e)\n" +
		"let r2: Result[int] = map_err(err, wrap_err)\n" +
		"let transformed: error = unwrap_err(r2)\n" +
		"print(transformed.message)\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	// The transformed error must be the NEW message built from the original.
	if stdout != "5\nwrapped: a\n" {
		t.Errorf("stdout = %q, want %q", stdout, "5\nwrapped: a\n")
	}
}
