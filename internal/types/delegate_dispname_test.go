package types

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestDelegateDispName_ArityErrors covers one representative case per Group-B
// shape (plain literal-hardcoded function) plus the Group-A shapes
// (checkMinMaxCall, checkFindAnyAllCall via higherOrderArgs, checkFirstLastCall
// via arrayBuiltinArg, checkTakeDropCall), asserting the diagnostic names the
// module-qualified spelling, not the dead flat key.
func TestDelegateDispName_ArityErrors(t *testing.T) {
	for _, c := range []struct {
		ns, src, want string
	}{
		{"math", `fn main() -> int { let i: int = math.abs(1, 2); return 0 }`, "math.abs expects 1 argument, got 2"},
		{"math", `fn main() -> int { let i: int = math.min(1); return 0 }`, "math.min expects 2 arguments, got 1"},
		{"array", `fn main() -> int { let xs: int[] = [1]; let f: int = array.first(xs, xs); return 0 }`, "array.first expects 1 argument, got 2"},
		{"array", `fn main() -> int { let xs: int[] = [1]; let ys: int[] = array.take(xs); return 0 }`, "array.take expects 2 arguments, got 1"},
		{"string", `fn main() -> int { let i: int = string.join([1]); return 0 }`, "string.join expects 2 arguments, got 1"},
		{"dict", `fn main() -> int { let m: {string: int} = {}; let g: int = dict.get_or(m); return 0 }`, "dict.get_or expects"},
		{"process", `fn main() -> int { let p: Process = process.spawn(); return 0 }`, "process.spawn expects"},
	} {
		info := checkNsProg(t, c.ns, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}

// TestDelegateDispName_RegressionGuards pins the two spec-required
// unchanged-behavior cases (spec Test Plan item 1): a coreSig member's
// diagnostic (already correct, must not change) and a genuinely flat
// non-modularized builtin's diagnostic (dispName == n.CalleeName, must not
// change).
func TestDelegateDispName_RegressionGuards(t *testing.T) {
	mathInfo := checkMathProg(t, `fn main() -> int { let f: float = math.sqrt("x"); return 0 }`)
	if !hasErr(mathInfo, "argument 1 of math.sqrt has type string, want float") {
		t.Errorf("coreSig regression: want math.sqrt error unchanged, got %v", errMsgs(mathInfo))
	}
	expectErr(t, wrapMain(`print()`), "print expects 1 to 2 arguments, got 0")
}

// The exhaustive negative check ("no diagnostic still names the bare flat
// key") is NOT duplicated here — Step 7's TestDelegateDispName_ExhaustiveAritySweep
// below performs that check correctly (with the spec's exact regexp) across
// every delegate member, which is a stronger and non-redundant guarantee than
// a per-case ad hoc check in this test could give.

// TestDelegateDispName_HigherOrderArgError covers checkFindAnyAllCall's
// higherOrderArgs-mediated diagnostic and checkMapCall/checkFilterCall/
// checkEachCall's own higherOrderArgs call sites.
func TestDelegateDispName_HigherOrderArgError(t *testing.T) {
	for _, c := range []struct {
		src, want string
	}{
		{`fn id(x: int) -> int { return x }
fn main() -> int { let xs: int[] = [1]; let r: Optional[int] = array.find(xs, id); return 0 }`, "argument 2 of array.find must return bool"},
		{`fn id(x: int) -> int { return x }
fn main() -> int { let xs: int[] = [1]; let ys: int[] = array.filter(xs, id); return 0 }`, "filter must return bool"},
		{`fn id(x: int) -> int { return x }
fn main() -> int { array.each([1], id); return 0 }`, "each must return void"},
	} {
		info := checkNsProg(t, "array", c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}

// TestDelegateDispName_SharedArrayHelper covers checkSortCall's
// arrayBuiltinArg-mediated diagnostic naming the module-qualified spelling.
func TestDelegateDispName_SharedArrayHelper(t *testing.T) {
	info := checkNsProg(t, "array", `fn main() -> int { let m: {string: int} = {}; array.sort(m); return 0 }`)
	if !hasErr(info, "argument 1 of array.sort must be an array") {
		t.Errorf("want array.sort in diagnostic, got %v", errMsgs(info))
	}
	if hasErr(info, "argument 1 of sort must be an array") {
		t.Errorf("diagnostic still names bare flat key, got %v", errMsgs(info))
	}
}

// TestDelegateDispName_ExhaustiveAritySweep iterates every delegate:true
// coreCatalog member, calls it with zero arguments (triggering an arity
// error for every shape — checkBuiltinSig-fallthrough members like gcd/random
// included), and asserts the diagnostic names ns.member and never the bare
// flat key as a leading token. The regexp is built exactly as the spec
// specifies: character class matches whitespace, "(", or a literal double
// quote (three alternatives), immediately before the flat key.
func TestDelegateDispName_ExhaustiveAritySweep(t *testing.T) {
	quote := "\""
	for ns, members := range coreCatalog {
		if strings.HasPrefix(ns, "__") {
			continue // test-sentinel namespaces (__probe/__dprobe) injected by other test files
		}
		for member, m := range members {
			if !m.delegate {
				continue
			}
			full := ns + "." + member
			src := fmt.Sprintf("fn main() -> int { %s(); return 0 }", full)
			info := checkNsProg(t, ns, src)
			if len(info.Errors) == 0 {
				t.Errorf("%s(): want an arity error, got none", full)
				continue
			}
			leadingFlatKey := regexp.MustCompile(`(^|[\s(` + quote + `])` + regexp.QuoteMeta(member) + `\b`)
			found := false
			for _, e := range info.Errors {
				if strings.Contains(e.Msg, full) {
					found = true
				}
				// Flag a leading bare flat key unconditionally — even if the same
				// message also happens to contain the qualified spelling
				// elsewhere, a leading bare flat key is still a residual bug (the
				// regexp's character class already excludes a "." immediately
				// before the key, so a true "ns.key" occurrence never matches
				// here; an earlier draft's extra "&& !strings.Contains(..., full)"
				// guard could mask a real leading-bare-key bug and was removed —
				// caught during plan review, Testability lens round 1).
				if leadingFlatKey.MatchString(e.Msg) {
					t.Errorf("%s(): diagnostic %q names the bare flat key %q, not %q", full, e.Msg, member, full)
				}
			}
			if !found {
				t.Errorf("%s(): no diagnostic contains %q, got %v", full, full, errMsgs(info))
			}
		}
	}
}

// TestDelegateDispName_SharedHelperCallSites covers every distinct
// delegate-reachable call site into arrayBuiltinArg, dictBuiltinArg, and
// higherOrderArgs, triggering each HELPER's own wrong-type error (not an
// arity error, which would return before the helper is ever reached) and
// asserting the message names ns.member and never the bare flat member name
// (spec Test Plan item 4's explicit negative check).
func TestDelegateDispName_SharedHelperCallSites(t *testing.T) {
	for _, c := range []struct {
		ns, member, src, wantSuffix string
	}{
		// arrayBuiltinArg callers
		{"array", "sort", `fn main() -> int { let m: {string: int} = {}; array.sort(m); return 0 }`, "must be an array"},
		{"array", "sum", `fn main() -> int { let m: {string: int} = {}; let s: int = array.sum(m); return 0 }`, "must be an array"},
		{"array", "first", `fn main() -> int { let m: {string: int} = {}; let f: int = array.first(m); return 0 }`, "must be an array"},
		{"array", "pop", `fn main() -> int { let m: {string: int} = {}; let p: int = array.pop(m); return 0 }`, "must be an array"},
		// dictBuiltinArg caller
		{"dict", "values", `fn main() -> int { let xs: int[] = [1]; let v: int[] = dict.values(xs); return 0 }`, "must be a dict"},
		// higherOrderArgs callers (non-arity path: wrong first-argument type)
		{"array", "find", "fn id(x: int) -> int { return x }\nfn main() -> int { let m: {string: int} = {}; let r: Optional[int] = array.find(m, id); return 0 }", "must be an array"},
		{"array", "map", "fn id(x: int) -> int { return x }\nfn main() -> int { let m: {string: int} = {}; let ys: int[] = array.map(m, id); return 0 }", "must be an array"},
		{"array", "filter", "fn id(x: int) -> int { return x }\nfn main() -> int { let m: {string: int} = {}; let ys: int[] = array.filter(m, id); return 0 }", "must be an array"},
		{"array", "each", "fn id(x: int) -> int { return x }\nfn main() -> int { let m: {string: int} = {}; array.each(m, id); return 0 }", "must be an array"},
		{"array", "count_where", "fn id(x: int) -> int { return x }\nfn main() -> int { let m: {string: int} = {}; let n: int = array.count_where(m, id); return 0 }", "must be an array"},
	} {
		full := c.ns + "." + c.member
		want := "argument 1 of " + full + " " + c.wantSuffix
		bareFlatKey := "argument 1 of " + c.member + " " + c.wantSuffix
		info := checkNsProg(t, c.ns, c.src)
		if !hasErr(info, want) {
			t.Errorf("%s: want %q, got %v", c.src, want, errMsgs(info))
		}
		if hasErr(info, bareFlatKey) {
			t.Errorf("%s: diagnostic still names the bare flat key %q, got %v", c.src, bareFlatKey, errMsgs(info))
		}
	}
}
