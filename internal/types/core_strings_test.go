package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// checkStringsProg checks a root program that imports string as namespace
// "string" (bound to a synthetic core module at id 1). Mirrors checkMathProg.
func checkStringsProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"string": 1})
	sm := coreMod(1, "string")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, sm}})
}

func TestCoreStringsCoreSigMembers(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let s: string = string.lower("A"); return 0 }`, "lower", String},
		{`fn main() -> int { let s: string = string.upper("a"); return 0 }`, "upper", String},
		{`fn main() -> int { let s: string = string.trim(" a "); return 0 }`, "trim", String},
		{`fn main() -> int { let s: string = string.trim_start(" a"); return 0 }`, "trim_start", String},
		{`fn main() -> int { let s: string = string.trim_end("a "); return 0 }`, "trim_end", String},
		{`fn main() -> int { let s: string = string.trim_prefix("ab", "a"); return 0 }`, "trim_prefix", String},
		{`fn main() -> int { let s: string = string.trim_suffix("ab", "b"); return 0 }`, "trim_suffix", String},
		{`fn main() -> int { let s: string = string.replace("a", "a", "b"); return 0 }`, "replace", String},
		{`fn main() -> int { let s: string = string.replace_first("aa", "a", "b"); return 0 }`, "replace_first", String},
		{`fn main() -> int { let a: string[] = string.split("a,b", ","); return 0 }`, "split", arrayType(String)},
		{`fn main() -> int { let b: bool = string.starts_with("ab", "a"); return 0 }`, "starts_with", Bool},
		{`fn main() -> int { let b: bool = string.ends_with("ab", "b"); return 0 }`, "ends_with", Bool},
		{`fn main() -> int { let s: string = string.substring("abc", 0, 2); return 0 }`, "substring", String},
		{`fn main() -> int { let s: string = string.char_at("abc", 1); return 0 }`, "char_at", String},
		{`fn main() -> int { let o: Optional[int] = string.last_index_of("abc", "b"); return 0 }`, "last_index_of", optionalType(Int)},
		{`fn main() -> int { let i: int = string.count("aaa", "a"); return 0 }`, "count", Int},
		{`fn main() -> int { let s: string = string.pad_start("a", 3, " "); return 0 }`, "pad_start", String},
		{`fn main() -> int { let s: string = string.pad_end("a", 3, " "); return 0 }`, "pad_end", String},
		{`fn main() -> int { let a: string[] = string.lines("a\nb"); return 0 }`, "lines", arrayType(String)},
		{`fn main() -> int { let b: bool = string.is_empty(""); return 0 }`, "is_empty", Bool},
		{`fn main() -> int { let s: string = string.reverse("abc"); return 0 }`, "reverse_string", String},
		{`fn main() -> int { let i: int = string.ord("a"); return 0 }`, "ord", Int},
		{`fn main() -> int { let i: int = string.int_or("x", 0); return 0 }`, "int_or", Int},
		{`fn main() -> int { let f: float = string.float_or("x", 0.0); return 0 }`, "float_or", Float},
	} {
		info := checkStringsProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

func TestCoreStringsDelegateMembers(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let s: string = string.join(["a", "b"], ","); return 0 }`, "join", String},
		{`fn main() -> int { let b: bool = string.contains("ab", "a"); return 0 }`, "contains", Bool},
		{`fn main() -> int { let o: Optional[int] = string.index_of("ab", "b"); return 0 }`, "index_of", optionalType(Int)},
		{`fn main() -> int { let s: string = string.repeat("x", 3); return 0 }`, "repeat", String},
		{`fn main() -> int { let s: string = string.chr(65); return 0 }`, "chr", String},
		{`fn main() -> int { let s: string = string.format_float(1.5, 2); return 0 }`, "format_float", String},
	} {
		info := checkStringsProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// TestCoreStringsContainsArrayOverload documents the harmless array reach-through:
// string.contains/index_of delegate to the flat overload resolver, so the array
// form type-checks exactly as the flat builtin would.
func TestCoreStringsContainsArrayOverload(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { let xs: int[] = [1, 2]; let b: bool = string.contains(xs, 1); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("string.contains array form: unexpected errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "contains"); ci == nil || ci.Result != Bool {
		t.Errorf("string.contains array form: got %v, want Bool", ci)
	}
}

// TestCoreStringsReverseMapsToReverseString pins the reverse -> reverse_string
// rename: the recorded Builtin key is reverse_string, not reverse.
func TestCoreStringsReverseMapsToReverseString(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { let s: string = string.reverse("abc"); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("string.reverse: unexpected errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "reverse_string"); ci == nil {
		t.Fatal("string.reverse did not record Builtin reverse_string")
	}
	if ci := callWithBuiltin(info, "reverse"); ci != nil {
		t.Errorf("string.reverse must NOT record Builtin reverse (that is the array builtin)")
	}
}

// TestCoreStringsArgTypeErrorNamesMember: a coreSig member's arg-type diagnostic
// names string.member (checkBuiltinSig uses dispName).
func TestCoreStringsArgTypeErrorNamesMember(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { let s: string = string.upper(1); return 0 }`)
	if !hasErr(info, "argument 1 of string.upper has type int, want string") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

// TestCoreStringsDelegateDomainErrorFlatNamed: a delegate member preserves the
// compile-time arg-domain diagnostic (flat-named, as the reused handler hardcodes).
func TestCoreStringsDelegateDomainErrorFlatNamed(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{`fn main() -> int { let s: string = string.repeat("x", -1); return 0 }`, "repeat(): negative count"},
		{`fn main() -> int { let s: string = string.chr(0); return 0 }`, "chr(): code out of range 1-255"},
		{`fn main() -> int { let s: string = string.format_float(1.0, -1); return 0 }`, "format_float: decimals must be >= 0"},
	} {
		info := checkStringsProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Fatalf("%q: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}

// TestCoreStringsFuncrefValueAllowed pins Part 3: string.trim (a generatable
// builtin) IS referenceable as a funcref VALUE and records a MemberFuncRef.
func TestCoreStringsFuncrefValueAllowed(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { let f: fn(string)->string = string.trim; let _: string = f(" x "); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("string.trim funcref should type-check; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for string.trim")
	}
}

func TestCoreStringsFuncrefValue_ArrayBoolArm_Unmatched(t *testing.T) {
	// F9 flagship: an annotation shaped like a funcref, matching no arm of
	// contains (pinned to int[] only, not bool[]).
	info := checkStringsProg(t, `fn main() -> int {
  let f: fn(bool[], bool) -> bool = string.contains
  return 0
}`)
	want := `"contains" has no function-reference form matching fn([bool],bool)->bool; supported: fn(string,string)->bool, fn([int],int)->bool`
	if !hasErr(info, want) {
		t.Fatalf("want %q, got %v", want, errMsgs(info))
	}
}

func TestCoreStringsFuncrefValue_ArrayIntArm_OK(t *testing.T) {
	// Positive control: the supported int[] arm still resolves clean,
	// proving the fix does not widen rejection to the already-supported arm.
	info := checkStringsProg(t, `fn main() -> int {
  let f: fn(int[], int) -> bool = string.contains
  return 0
}`)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded")
	}
}

func TestCoreStringsTypeArgsRejected(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { let s: string = string.upper[int]("a"); return 0 }`)
	if !hasErr(info, "string.upper does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreStringsUnknownMemberSuggestion(t *testing.T) {
	info := checkStringsProg(t, `fn main() -> int { string.uppe("a"); return 0 }`)
	if !hasErr(info, `did you mean "upper"?`) {
		t.Fatalf("want upper suggestion, got %v", errMsgs(info))
	}
}

func TestCoreStringsAliasImport(t *testing.T) {
	// import "string" as s -> s.lower resolves identically to string.lower.
	root := mod(t, 0, `fn main() -> int { let x: string = s.lower("A"); return 0 }`, map[string]int{"s": 1})
	sm := coreMod(1, "string")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, sm}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased s.lower should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "lower"); ci == nil || ci.Result != String {
		t.Errorf("s.lower result = %v, want string", ci)
	}
}

func TestCoreStringsMissingImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let x: string = string.lower("A"); return 0 }`, nil)
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root}})
	if !hasErr(info, `module "string" is not imported; add import "string"`) {
		t.Fatalf("want undeclared-name error, got %v", errMsgs(info))
	}
}
