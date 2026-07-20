package types

import "testing"

// now() -> int and sleep(secs: int) -> Void checker coverage.
// Positive cases assert result types; negative cases assert located errors for
// wrong arity, wrong argument type, Void-as-value, and reserved names.

// --- well-typed results ---

func TestTime_Now_OK(t *testing.T) {
	expectOK(t, wrapMain(`let t: int = now()`))
}

func TestTime_Sleep_OK(t *testing.T) {
	expectOK(t, wrapMain(`sleep(0)`))
	expectOK(t, wrapMain(`sleep(5)`))
}

// --- now() arity ---

func TestTime_Now_TakesNoArgs(t *testing.T) {
	expectErr(t, wrapMain(`let t: int = now(1)`), "now")
}

// --- sleep() arity and type ---

func TestTime_Sleep_TakesOneArg(t *testing.T) {
	expectErr(t, wrapMain(`sleep()`), "sleep")
	expectErr(t, wrapMain(`sleep(1, 2)`), "sleep")
}

func TestTime_Sleep_ArgMustBeInt(t *testing.T) {
	expectErr(t, wrapMain(`sleep("s")`), "sleep")
}

// --- Void result: sleep() may not appear as a value ---

func TestTime_Sleep_VoidNotAValue(t *testing.T) {
	expectErr(t, wrapMain(`let x: int = sleep(1)`), "void")
}

// --- reserved names ---

func TestTime_NowSleep_ReservedNames_Fn(t *testing.T) {
	for _, name := range []string{"now", "sleep"} {
		src := "fn " + name + "() -> int {\n  return 0\n}\nfn main() -> int {\n  return 0\n}"
		expectErr(t, src, name)
	}
}

func TestTime_NowSleep_ReservedNames_Let(t *testing.T) {
	for _, name := range []string{"now", "sleep"} {
		expectErr(t, wrapMain("let "+name+": int = 0"), "reserved")
	}
}

func TestTime_NowSleep_ReservedNames_Param(t *testing.T) {
	for _, name := range []string{"now", "sleep"} {
		src := "fn f(" + name + ": int) -> int {\n  return 0\n}\nfn main() -> int {\n  return 0\n}"
		expectErr(t, src, name)
	}
}

// --- math.random(max: int) -> int ---
//
// random(max) is a removable builtin now spelled math.random; its OK
// resolution and domain check (max must be positive) also live in
// core_math_test.go (TestCoreMathCoreSigMembers / TestCoreMathDomainFaults),
// but the arity/arg-type/name-freed coverage below has no other home.

func TestRandom_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let r: int = math.random(100)`), "math")
	expectOKNS(t, wrapMain(`let r: int = math.random(1)`), "math")
}

func TestRandom_TakesOneArg(t *testing.T) {
	// random delegates to the flat dispatch path (composite-arg special case),
	// so the diagnostic names the flat builtin, not the member path.
	expectErrNS(t, wrapMain(`let r: int = math.random()`), "random", "math")
	expectErrNS(t, wrapMain(`let r: int = math.random(1, 2)`), "random", "math")
}

func TestRandom_ArgMustBeInt(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: int = math.random("s")`), "random", "math")
}

// random is a removable builtin: its flat name was freed by the
// modules-only migration (isReservedName excludes the removable set), so it
// is now an ordinary identifier a user may bind -- unlike the pre-removal
// original, which reserved it as a bare builtin name.
func TestRandom_NameFreed_Fn(t *testing.T) {
	src := "fn random() -> int {\n  return 0\n}\nfn main() -> int {\n  return 0\n}"
	expectOKNS(t, src, "math")
}

func TestRandom_NameFreed_Let(t *testing.T) {
	expectOKNS(t, wrapMain(`let random: int = 0`), "math")
}

func TestRandom_NameFreed_Param(t *testing.T) {
	src := "fn f(random: int) -> int {\n  return 0\n}\nfn main() -> int {\n  return 0\n}"
	expectOKNS(t, src, "math")
}
