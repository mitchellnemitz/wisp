package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

// TestBuiltinFuncref_AC0_AC1 is the checker-only acceptance test for the
// builtin-funcrefs allowlist (Task 1).  The wrappers (__wisp_builtin_*) do not
// exist yet, so these tests assert only CHECKER behaviour -- diagnostics and
// inferred types -- without compiling or running generated shell.
func TestBuiltinFuncref_AC0_AC1(t *testing.T) {
	// AC0: an allowlisted stays-flat builtin is accepted in value position with
	// the correct fn type. Removable-builtin funcref acceptance (trim, sqrt,
	// is_file, log10, program_path, ...) is covered by the namespaced core_*_test
	// suites (TestCoreStringsFuncrefValueAllowed, TestCoreMathFuncrefValueAllowed,
	// TestCoreFSFuncrefValueAllowed/ProgramPath, ...).
	expectOK(t, `fn main()->int{ let f: fn()->int = now; return f() }`)

	// AC1: stays-flat non-members are rejected with a reason-branched message.
	// The removable-builtin rejection variants (map/filter "container overload",
	// abs/min "overloaded", run/spawn/split/which "funcref-shaped") are covered by
	// the core_*_test suites.
	// special-cased overload (length: string|array -- builtinOverloaded):
	expectErr(t, `fn main()->int{ let f: fn(string)->int = length; return 0 }`, "overloaded")
	// statement/void:
	expectErr(t, `fn main()->int{ let f: fn(string)->void = print; return 0 }`, "statement")
	// no-single-funcref-shaped-scalar-lowering (read_line: Optional/nullary):
	expectErr(t, `fn main()->int{ let f: fn(string)->string = read_line; return 0 }`, "funcref-shaped")
}

// TestBuiltinFuncref_OverloadedArgContextGenericCallee pins that a concrete
// (non-type-variable) parameter of a GENERIC callee still propagates its type
// as call-argument context, even though other parameters of that same callee
// remain unresolved type variables at check time. Uses the namespaced overloaded
// member math.abs (the bare abs surface is removed); the non-generic-callee
// variant is TestCoreMathFuncrefValueAbsArgContext.
func TestBuiltinFuncref_OverloadedArgContextGenericCallee(t *testing.T) {
	info := checkMathProg(t, `fn apply[T](f: fn(int)->int, x: T) -> int { return f(-3) }
fn main() -> int { return apply(math.abs, 1) }`)
	if len(info.Errors) != 0 {
		t.Fatalf("math.abs as a concrete-typed argument of a generic callee should resolve via that parameter's type; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for math.abs")
	}
}

// TestBuiltinFuncref_OverloadedArms pins the full int/float overload-arm matrix
// for math.abs/min/max/clamp/sign: a matching-arm annotation resolves and
// records a MemberFuncRef of exactly that arm's type; a non-matching (or
// absent) annotation is rejected as ambiguous, not silently coerced to some
// default arm. Uses the namespaced spelling (the bare abs/min/max/clamp/sign
// surface is removed).
func TestBuiltinFuncref_OverloadedArms(t *testing.T) {
	for _, c := range []struct {
		name string
		ok   string // an OK case: `let f: <T> = name; ...` snippet body
	}{
		{"abs", `fn main()->int{ let f: fn(int)->int = math.abs; let _: int = f(-3); return 0 }`},
		{"abs", `fn main()->int{ let f: fn(float)->float = math.abs; let _: float = f(-3.0); return 0 }`},
		{"min", `fn main()->int{ let f: fn(int,int)->int = math.min; let _: int = f(1,2); return 0 }`},
		{"min", `fn main()->int{ let f: fn(float,float)->float = math.min; let _: float = f(1.0,2.0); return 0 }`},
		{"max", `fn main()->int{ let f: fn(int,int)->int = math.max; let _: int = f(1,2); return 0 }`},
		{"max", `fn main()->int{ let f: fn(float,float)->float = math.max; let _: float = f(1.0,2.0); return 0 }`},
		{"clamp", `fn main()->int{ let f: fn(int,int,int)->int = math.clamp; let _: int = f(5,0,10); return 0 }`},
		{"clamp", `fn main()->int{ let f: fn(float,float,float)->float = math.clamp; let _: float = f(5.0,0.0,10.0); return 0 }`},
		{"sign", `fn main()->int{ let f: fn(int)->int = math.sign; let _: int = f(-3); return 0 }`},
		{"sign", `fn main()->int{ let f: fn(float)->int = math.sign; let _: int = f(-3.0); return 0 }`},
	} {
		info := checkMathProg(t, c.ok)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: %q: unexpected errors: %v", c.name, c.ok, errMsgs(info))
		}
		if len(info.MemberFuncRefs) == 0 {
			t.Fatalf("%s: %q: expected a MemberFuncRef recorded", c.name, c.ok)
		}
	}

	// Ambiguous: a context matching no arm is rejected, naming the arms.
	for _, c := range []struct{ src, want string }{
		{`fn main()->int{ let f: fn(bool)->bool = math.abs; return 0 }`,
			`"abs" has no function-reference form matching fn(bool)->bool; supported: fn(int)->int, fn(float)->float`},
		{`fn main()->int{ let f: fn(int)->int = math.min; return 0 }`, // arity mismatch: min takes 2
			`"min" has no function-reference form matching fn(int)->int; supported: fn(int,int)->int, fn(float,float)->float`},
		{`fn main()->int{ let f: fn(string)->string = math.max; return 0 }`,
			`"max" has no function-reference form matching fn(string)->string; supported: fn(int,int)->int, fn(float,float)->float`},
		{`fn main()->int{ let f: fn(int,int)->int = math.clamp; return 0 }`, // arity mismatch: clamp takes 3
			`"clamp" has no function-reference form matching fn(int,int)->int; supported: fn(int,int,int)->int, fn(float,float,float)->float`},
		{`fn main()->int{ let f: fn(bool)->bool = math.sign; return 0 }`,
			`"sign" has no function-reference form matching fn(bool)->bool; supported: fn(int)->int, fn(float)->int`},
	} {
		info := checkMathProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%q: expected %q, got: %v", c.src, c.want, errMsgs(info))
		}
	}
}

// TestBuiltinFuncref_OverloadedArgContext pins that a namespaced overloaded
// builtin funcref (math.abs) is disambiguated by the expected parameter type at
// a call-argument reference site, not only by a `let` annotation (design spec
// Part 2: "a funcref-typed annotation or the expected parameter type at the
// reference site"). Uses the namespaced spelling (the bare abs surface is
// removed); see TestCoreMathFuncrefValueAbsArgContext for another angle on the
// same invariant.
func TestBuiltinFuncref_OverloadedArgContext(t *testing.T) {
	info := checkMathProg(t, `fn apply(f: fn(int)->int) -> int { return f(-3) }
fn main()->int{ return apply(math.abs) }`)
	if len(info.Errors) != 0 {
		t.Fatalf("math.abs as a call argument should resolve via the parameter's expected type; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for math.abs")
	}
}

// TestBuiltinFuncref_AllowlistConsistency verifies that every generatable member
// satisfies the uniform-wrapper TYPE necessary condition: a builtinSigs entry
// where every param has exactly one scalar type and the result is scalar or
// void. This catches a wrongly-added overloaded, composite, or handle-typed
// member. (The result may be void here, unlike the old scalar-only allowlist,
// because the void category is now generatable.)
func TestBuiltinFuncref_AllowlistConsistency(t *testing.T) {
	for name := range builtinFuncrefGeneratable {
		if _, ok := builtinSigs[name]; !ok {
			t.Fatalf("generatable member %q not in builtinSigs", name)
		}
		if !builtinFuncrefGeneratableSig(name) {
			t.Errorf("generatable member %q has a non-uniform signature %v -> %q",
				name, builtinSigs[name].params, builtinSigs[name].result)
		}
	}
}

// TestBuiltinFuncref_NullaryReferenceable pins that a nullary total-helper
// builtin is referenceable and records its fn()->T funcref type.
func TestBuiltinFuncref_NullaryReferenceable(t *testing.T) {
	// now stays flat. pi and program_path are covered by
	// TestCoreMathFuncrefValuePi and TestCoreFSFuncrefValueProgramPath; fs.cwd (the
	// namespaced spelling; the bare cwd surface is removed) is pinned here.
	expectOK(t, `fn main()->int{ let f: fn()->int = now; return f() }`)
	info := checkFSProg(t, `fn main()->int{ let f: fn()->string = fs.cwd; return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("fs.cwd as a nullary funcref value: unexpected errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for fs.cwd")
	}
}

// TestBuiltinFuncref_OverloadedHigherOrderArgContext pins that a namespaced
// overloaded builtin funcref (math.abs) passed directly as the second argument
// of array.map is disambiguated from the array/Optional/Result element type,
// not only by a `let` annotation or a user-function call argument (see
// TestBuiltinFuncref_OverloadedArgContext for the latter). This exercises
// higherOrderArgs and checkMapCall's Optional/Result probe branch, which have
// their own checkExprExpecting call sites distinct from checkUserCallIn. Uses
// math.abs and array.map (the bare map/abs surface is removed); the funcref is
// recorded in MemberFuncRefs, not FuncRefs, for a namespaced reference (see
// resolveQualifiedConst).
func TestBuiltinFuncref_OverloadedHigherOrderArgContext(t *testing.T) {
	for _, src := range []string{
		`fn main()->int{ let xs: int[] = [-1, 2]; let ys: int[] = array.map(xs, math.abs); return ys[0] }`,
		`fn main()->int{ let o: Optional[int] = Some(-3); let r: Optional[int] = array.map(o, math.abs); return 0 }`,
		`fn main()->int{ let r: Result[int] = Ok(-3); let r2: Result[int] = array.map(r, math.abs); return 0 }`,
	} {
		root := mod(t, 0, src, map[string]int{"math": 1, "array": 2})
		info := CheckLinked(&module.Linked{Modules: []*module.Module{root, coreMod(1, "math"), coreMod(2, "array")}})
		if len(info.Errors) != 0 {
			t.Fatalf("%q: unexpected errors: %v", src, errMsgs(info))
		}
		if len(info.MemberFuncRefs) == 0 {
			t.Fatalf("%q: expected a MemberFuncRef recorded for math.abs", src)
		}
	}
}
