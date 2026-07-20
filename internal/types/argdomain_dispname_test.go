package types

import "testing"

// TestArgDomainDispName covers every coreCatalog-delegate member present in
// builtinIntArgDomains (all 9 entries except sleep, which is flat-only and thus
// excluded — see Global Constraints; wait_any is covered separately below),
// asserting each domain-check diagnostic names the module-qualified spelling
// and never the bare flat key (spec Test Plan item 3's explicit negative check).
func TestArgDomainDispName(t *testing.T) {
	for _, c := range []struct {
		ns, src, want, bareFlatKey string
	}{
		{"math", `fn main() -> int { let i: int = math.abs(-9223372036854775808); return 0 }`, "math.abs(): integer overflow", "abs(): integer overflow"},
		{"math", `fn main() -> int { let i: int = math.gcd(-9223372036854775808, 1); return 0 }`, "math.gcd(): integer overflow", "gcd(): integer overflow"},
		{"math", `fn main() -> int { let i: int = math.random(0); return 0 }`, "math.random: max must be positive", "random: max must be positive"},
		{"string", `fn main() -> int { let s: string = string.repeat("x", -1); return 0 }`, "string.repeat(): negative count", "repeat(): negative count"},
		{"string", `fn main() -> int { let s: string = string.chr(0); return 0 }`, "string.chr(): code out of range 1-255", "chr(): code out of range 1-255"},
		{"string", `fn main() -> int { let s: string = string.format_float(1.0, -1); return 0 }`, "string.format_float: decimals must be >= 0", "format_float: decimals must be >= 0"},
		{"array", `fn main() -> int { let xs: int[] = [1]; array.remove_at(xs, -1); return 0 }`, "array.remove_at: index out of range", "remove_at: index out of range"},
		{"array", `fn main() -> int { let xs: int[] = [1]; array.insert_at(xs, -1, 9); return 0 }`, "array.insert_at: index out of range", "insert_at: index out of range"},
	} {
		info := checkNsProg(t, c.ns, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want %q, got %v", c.src, c.want, errMsgs(info))
		}
		for _, e := range info.Errors {
			if e.Msg == c.bareFlatKey {
				t.Errorf("%s: diagnostic still uses the bare flat key %q verbatim", c.src, c.bareFlatKey)
			}
		}
	}
}

// TestArgDomainDispName_WaitAny covers process.wait_any, which needs a Process[]
// receiver constructed via process.spawn first.
func TestArgDomainDispName_WaitAny(t *testing.T) {
	info := checkNsProg(t, "process", `fn main() -> int { let p: Process = process.spawn(["x"]); let ps: Process[] = [p]; let w: Process = process.wait_any(ps, -1); return 0 }`)
	if !hasErr(info, "process.wait_any: poll_secs must be >= 0") {
		t.Fatalf("want module-named wait_any domain error, got %v", errMsgs(info))
	}
	for _, e := range info.Errors {
		if e.Msg == "wait_any: poll_secs must be >= 0" {
			t.Fatalf("diagnostic still uses the bare flat key verbatim: %q", e.Msg)
		}
	}
}

// TestArgDomainDispName_SleepUnaffected pins that sleep, a flat-only builtin not
// in coreCatalog, keeps its unqualified message (dispName == name for flat calls).
func TestArgDomainDispName_SleepUnaffected(t *testing.T) {
	info := expectErr(t, wrapMain(`sleep(-1)`), "sleep: negative duration")
	if info.Pos.Line == 0 {
		t.Fatalf("sleep domain error not located")
	}
}
