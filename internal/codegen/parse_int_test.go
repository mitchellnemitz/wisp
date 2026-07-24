package codegen

import (
	"strings"
	"testing"
)

// parse_int lowers through the same Optional-sentinel path as which/read_link:
// call the helper, capture $? immediately, then branch into Some(__ret)/None.
func TestParseIntLoweringShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let o: Optional[int] = parse_int("42")`)))
	if !strings.Contains(out, "__wisp_parse_int") {
		t.Errorf("parse_int: expected a call to __wisp_parse_int, got:\n%s", out)
	}
	if !strings.Contains(out, "=$?") {
		t.Errorf("parse_int: expected the exit status captured immediately after the call, got:\n%s", out)
	}
	if !strings.Contains(out, "}_tag=") {
		t.Errorf("parse_int: expected an Optional handle (_tag field), got:\n%s", out)
	}
	if !strings.Contains(out, "=some") || !strings.Contains(out, "=none") {
		t.Errorf("parse_int: expected both the some and none tag branches, got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

func TestParseIntBehavioralEndToEnd(t *testing.T) {
	src := wrapMainCG(
		`let a: Optional[int] = parse_int("42")` + "\n" +
			`let b: Optional[int] = parse_int("abc")` + "\n" +
			`print(to_string(unwrap_or(a, -1)))` + "\n" +
			`print(to_string(is_none(b)))`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	want := "42\ntrue\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
