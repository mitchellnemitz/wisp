package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

func checkMathProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"math": 1})
	mm := coreMod(1, "math")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, mm}})
}

func TestCoreMathCoreSigMembers(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let f: float = math.sqrt(4.0); return 0 }`, "sqrt", Float},
		{`fn main() -> int { let f: float = math.pow(2.0, 3.0); return 0 }`, "pow", Float},
		{`fn main() -> int { let f: float = math.exp(1.0); return 0 }`, "exp", Float},
		{`fn main() -> int { let f: float = math.ln(2.0); return 0 }`, "ln", Float},
		{`fn main() -> int { let f: float = math.log10(10.0); return 0 }`, "log10", Float},
		{`fn main() -> int { let f: float = math.log2(8.0); return 0 }`, "log2", Float},
		{`fn main() -> int { let i: int = math.floor(1.5); return 0 }`, "floor", Int},
		{`fn main() -> int { let i: int = math.ceil(1.5); return 0 }`, "ceil", Int},
		{`fn main() -> int { let i: int = math.round(1.5); return 0 }`, "round", Int},
		{`fn main() -> int { let i: int = math.trunc(1.5); return 0 }`, "trunc", Int},
		{`fn main() -> int { let i: int = math.lcm(2, 3); return 0 }`, "lcm", Int},
		{`fn main() -> int { let f: float = math.pi(); return 0 }`, "pi", Float},
		{`fn main() -> int { let i: int = math.int_max(); return 0 }`, "int_max", Int},
		{`fn main() -> int { let i: int = math.int_min(); return 0 }`, "int_min", Int},
	} {
		info := checkMathProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

func TestCoreMathDelegateOverloads(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let i: int = math.abs(-5); return 0 }`, "abs", Int},
		{`fn main() -> int { let f: float = math.abs(-5.0); return 0 }`, "abs", Float},
		{`fn main() -> int { let i: int = math.min(1, 2); return 0 }`, "min", Int},
		{`fn main() -> int { let f: float = math.min(1.0, 2.0); return 0 }`, "min", Float},
		{`fn main() -> int { let i: int = math.max(1, 2); return 0 }`, "max", Int},
		{`fn main() -> int { let i: int = math.clamp(5, 1, 10); return 0 }`, "clamp", Int},
		{`fn main() -> int { let f: float = math.clamp(5.0, 1.0, 10.0); return 0 }`, "clamp", Float},
		{`fn main() -> int { let i: int = math.gcd(12, 8); return 0 }`, "gcd", Int},
		{`fn main() -> int { let i: int = math.random(5); return 0 }`, "random", Int},
	} {
		info := checkMathProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.src, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.src, ci, c.want)
		}
	}
}

func TestCoreMathSignResolves(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let i: int = math.sign(-3); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("math.sign: unexpected errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "sign"); ci == nil {
		t.Errorf("math.sign did not record CallBuiltin sign")
	}
}

func TestCoreMathDelegatePreservesDomainChecks(t *testing.T) {
	for _, c := range []struct {
		src  string
		want string
	}{
		{`fn main() -> int { let i: int = math.random(0); return 0 }`, "math.random: max must be positive"},
		{`fn main() -> int { let i: int = math.gcd(-9223372036854775808, 1); return 0 }`, "math.gcd(): integer overflow"},
		{`fn main() -> int { let i: int = math.abs(-9223372036854775808); return 0 }`, "math.abs(): integer overflow"},
	} {
		info := checkMathProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}

func TestCoreMathAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let f: float = m.sqrt(4.0); return 0 }`, map[string]int{"m": 1})
	mm := coreMod(1, "math")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, mm}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased m.sqrt should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "sqrt"); ci == nil || ci.Result != Float {
		t.Errorf("m.sqrt result = %v, want float", ci)
	}
}

func TestCoreMathCoreSigArgTypeNamesMember(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let f: float = math.sqrt("x"); return 0 }`)
	if !hasErr(info, "argument 1 of math.sqrt has type string, want float") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

func TestCoreMathTypeArgsRejected(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let f: float = math.sqrt[int](4.0); return 0 }`)
	if !hasErr(info, "math.sqrt does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreMathUnknownMemberSuggestion(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let f: float = math.sqt(4.0); return 0 }`)
	if !hasErr(info, `did you mean "sqrt"?`) {
		t.Fatalf("want sqrt suggestion, got %v", errMsgs(info))
	}
}

// TestCoreMathFuncrefValueAllowed pins Part 3 for math: a fixed-signature
// member (sqrt) AND a DELEGATE member whose builtin is generatable (gcd) are
// both referenceable as funcref values and record a MemberFuncRef. The delegate
// flag is irrelevant to funcref-ability: only the builtin's presence in the
// generatable allowlist (and !takesTypeArgs) matters.
func TestCoreMathFuncrefValueAllowed(t *testing.T) {
	for _, src := range []string{
		`fn main() -> int { let f: fn(float)->float = math.sqrt; let _: float = f(4.0); return 0 }`,
		`fn main() -> int { let f: fn(int,int)->int = math.gcd; let _: int = f(6,4); return 0 }`,
	} {
		info := checkMathProg(t, src)
		if len(info.Errors) != 0 {
			t.Fatalf("%q: math funcref should type-check; errors: %v", src, errMsgs(info))
		}
		if len(info.MemberFuncRefs) == 0 {
			t.Fatalf("%q: expected a MemberFuncRef recorded", src)
		}
	}
}

// TestCoreMathFuncrefValueAbsAmbiguous pins that math.abs (an overloaded
// delegate) given an annotation matching no arm is rejected as an
// unmatched-arm ambiguity naming the supported arms, not silently accepted.
func TestCoreMathFuncrefValueAbsAmbiguous(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let f: fn(bool)->bool = math.abs; return 0 }`)
	if !hasErr(info, `"abs" has no function-reference form matching fn(bool)->bool; supported: fn(int)->int, fn(float)->float`) {
		t.Fatalf("want ambiguous-overload error, got %v", errMsgs(info))
	}
}

func TestCoreMathFuncrefValueAbsNoContext(t *testing.T) {
	// True no-annotation case (want == Invalid): a bare statement-expression
	// reference reaches checkExpr with no expected type. Distinct from
	// TestCoreMathFuncrefValueAbsAmbiguous, whose fn(bool)->bool annotation
	// now routes to the unmatched-arm branch under the corrected guard.
	info := checkMathProg(t, `fn main() -> int { math.abs; return 0 }`)
	if !hasErr(info, `reference to overloaded builtin "abs" needs a function-reference type annotation to select an overload`) {
		t.Fatalf("want no-annotation error, got %v", errMsgs(info))
	}
}

// TestCoreMathFuncrefValueAbsArm pins that math.abs IS referenceable given a
// matching-arm annotation, the same as the bare-ident path.
func TestCoreMathFuncrefValueAbsArm(t *testing.T) {
	info := checkMathProg(t, `fn main() -> int { let f: fn(int)->int = math.abs; let _: int = f(-3); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("math.abs funcref should type-check; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for math.abs")
	}
}

// TestCoreMathFuncrefValueAbsArgContext pins that an overloaded builtin
// funcref (bare and namespaced) is disambiguated by the expected parameter
// type at a call-argument reference site, not only by a `let` annotation
// (design spec Part 2: "a funcref-typed annotation or the expected parameter
// type at the reference site").
func TestCoreMathFuncrefValueAbsArgContext(t *testing.T) {
	info := checkMathProg(t, `fn apply(f: fn(int)->int) -> int { return f(-3) }
fn main() -> int { return apply(math.abs) }`)
	if len(info.Errors) != 0 {
		t.Fatalf("math.abs as a call argument should resolve via the parameter's expected type; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for math.abs")
	}
}

// TestCoreMathFuncrefValuePi pins that math.pi (a nullary total-helper
// delegate, now generatable) is referenceable as a funcref value via the same
// allowlist Part 3 consults for math.sqrt/math.gcd.
func TestCoreMathFuncrefValuePi(t *testing.T) {
	src := `fn main() -> int { let f: fn()->float = math.pi; return 0 }`
	info := checkMathProg(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("%q: math.pi funcref should type-check; errors: %v", src, errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("%q: expected a MemberFuncRef recorded", src)
	}
}
