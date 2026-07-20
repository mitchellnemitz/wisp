package codegen

import (
	"strings"
	"testing"
)

// wrapMainCG puts a body inside a valid main returning 0.
func wrapMainCG(body string) string {
	return "fn main() -> int {\n" + body + "\nreturn 0\n}"
}

func TestSomeLoweringShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let o: Optional[int] = Some(7)`)))
	if !strings.Contains(out, "}_tag=") {
		t.Errorf("Some: expected a _tag field assignment, got:\n%s", out)
	}
	if !strings.Contains(out, "=some") {
		t.Errorf("Some: expected the tag value 'some', got:\n%s", out)
	}
	if !strings.Contains(out, "_value=") {
		t.Errorf("Some: expected a _value field assignment, got:\n%s", out)
	}
}

func TestNoneLoweringShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let o: Optional[int] = None`)))
	if !strings.Contains(out, "}_tag=") {
		t.Errorf("None: expected a _tag field assignment, got:\n%s", out)
	}
	if !strings.Contains(out, "=none") {
		t.Errorf("None: expected the tag value 'none', got:\n%s", out)
	}
	// Assert no ASSIGNMENT to the value field (not merely a mention of "_value"),
	// so the test stays valid if a future helper references the field name.
	if strings.Contains(out, "_value=") {
		t.Errorf("None: expected NO _value field assignment, got:\n%s", out)
	}
}

func TestUnwrapAbortShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = unwrap(o)`)))
	if !strings.Contains(out, "__wisp_fail") || !strings.Contains(out, "unwrap of None") {
		t.Errorf("unwrap: expected __wisp_fail with 'unwrap of None', got:\n%s", out)
	}
}

func TestOptionalBehavioralEndToEnd(t *testing.T) {
	src := wrapMainCG(
		`let o: Optional[int] = Some(7)` + "\n" +
			`print(to_string(is_some(o)))` + "\n" +
			`print(to_string(unwrap(o)))` + "\n" +
			`let n: Optional[int] = None` + "\n" +
			`print(to_string(is_none(n)))` + "\n" +
			`print(to_string(unwrap_or(n, -1)))` + "\n" +
			`print(to_string(unwrap_or(o, -1)))`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	want := "true\n7\ntrue\n-1\n7\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// dict.get is a removable builtin (bare get no longer resolves in the
// single-module check), so the two tests below compile through compileNS/runNS
// with the dict namespace bound.

func TestGetLoweringShape(t *testing.T) {
	out := string(compileNS(t, wrapMainCG(`let d: {string:int} = {"a": 1}`+"\n"+`let o: Optional[int] = dict.get(d, "a")`), "dict"))
	if !strings.Contains(out, "case ") || !strings.Contains(out, "esac") {
		t.Errorf("get: expected a case membership test, got:\n%s", out)
	}
	if !strings.Contains(out, "}_tag=") {
		t.Errorf("get: expected an Optional handle (_tag field), got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

func TestGetBehavioralEndToEnd(t *testing.T) {
	src := wrapMainCG(
		`let d: {string:int} = {"a": 1}` + "\n" +
			`print(to_string(unwrap_or(dict.get(d, "a"), 0)))` + "\n" +
			`print(to_string(unwrap_or(dict.get(d, "z"), 0)))` + "\n" +
			`print(to_string(is_some(dict.get(d, "a"))))` + "\n" +
			`print(to_string(is_none(dict.get(d, "z"))))`)
	stdout, _, code := runNS(t, src, "dict")
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	want := "1\n0\ntrue\ntrue\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestOptEqSingleLevelShape(t *testing.T) {
	// Single-level Optional[int] equality
	out := string(compile(t, wrapMainCG(
		`let a: Optional[int] = Some(1)`+"\n"+
			`let b: Optional[int] = Some(2)`+"\n"+
			`let r: bool = a == b`)))
	// Count TAG READS (`_tag"` from readHandleVar's `eval "__ret=\$..._tag"`),
	// NOT all `_tag` occurrences: `Some(...)` construction emits `_tag=` WRITES
	// that would satisfy a bare `_tag` count even if `==` never read the tags.
	if got := strings.Count(out, `_tag"`); got < 2 {
		t.Errorf("OptEq single: expected >= 2 tag READS (both operands), got %d:\n%s", got, out)
	}
	// On matching tags the some-branch must read a value to compare it.
	if !strings.Contains(out, `_value"`) {
		t.Errorf("OptEq single: expected a value read for the some-branch compare:\n%s", out)
	}
	// Every expansion must be double-quoted (injection safety)
	if strings.Contains(out, "[ $") {
		t.Errorf("OptEq single: found unquoted expansion in [ ] test:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

func TestOptEqNestedShape(t *testing.T) {
	// Nested Optional[Optional[int]] equality
	out := string(compile(t, wrapMainCG(
		`let a: Optional[Optional[int]] = Some(Some(1))`+"\n"+
			`let inner: Optional[int] = None`+"\n"+
			`let b: Optional[Optional[int]] = Some(inner)`+"\n"+
			`let r: bool = a == b`)))
	// Recursion check: count TAG READS (`_tag"`), not all `_tag` (construction
	// emits `_tag=` WRITES). A non-recursive inner-handle-id compare reads only
	// the 2 OUTER tags; genuine structural recursion reads 4 (outer pair + inner
	// pair, the inner reads dereferencing the inner handle ids). So < 4 tag reads
	// means the lowering did NOT recurse into the inner Optionals (AC10b).
	if got := strings.Count(out, `_tag"`); got < 4 {
		t.Errorf("OptEq nested: expected >= 4 tag READS (outer+inner each side), got %d:\n%s", got, out)
	}
	// Every expansion must be double-quoted
	if strings.Contains(out, "[ $") {
		t.Errorf("OptEq nested: found unquoted expansion:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

func TestOptionalNestedCompositeRoundTrip(t *testing.T) {
	src := wrapMainCG(
		`let xs: int[] = [1, 2, 3]` + "\n" +
			`print(to_string(length(unwrap(Some(xs)))))` + "\n" +
			`let i: Optional[Optional[int]] = Some(Some(7))` + "\n" +
			`print(to_string(unwrap(unwrap(i))))`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	want := "3\n7\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
