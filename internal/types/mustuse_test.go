package types

import (
	"strings"
	"testing"
)

func hasMustUseWarning(info *Info) bool {
	for _, w := range info.Warnings {
		if strings.Contains(w.Msg, "discarded") {
			return true
		}
	}
	return false
}

func TestMustUse_BareOptionalDiscard_Warns(t *testing.T) {
	info := check(t, wrapMain(`parse_int("x")`))
	if len(info.Errors) != 0 {
		t.Fatalf("must-use warning must not be an error, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning, got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_UnderscoreAssign_NoWarning(t *testing.T) {
	info := expectOK(t, wrapMain(`_ = parse_int("x")`))
	if hasMustUseWarning(info) {
		t.Fatalf("`_ = ...` opt-out must not warn, got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_LetUnderscore_NoWarning(t *testing.T) {
	info := expectOK(t, wrapMain(`let _: Optional[int] = parse_int("x")`))
	if hasMustUseWarning(info) {
		t.Fatalf("`let _: T = ...` opt-out must not warn, got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_DiscardedUserResultFn_Warns(t *testing.T) {
	src := `fn f() -> Result[int] { return Ok(1) }
` + wrapMain(`f()`)
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning for a discarded Result, got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_DiscardedWhich_Warns(t *testing.T) {
	info := checkNS(t, wrapMain(`fs.which("ls")`), "fs")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning for discarded fs.which(), got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_DiscardedEnv_Warns(t *testing.T) {
	info := checkNS(t, wrapMain(`env.get("HOME")`), "env")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning for discarded env.get(), got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_DiscardedDictGet_Warns(t *testing.T) {
	src := wrapMain(`let d: {string:int} = {"a": 1}
dict.get(d, "a")`)
	info := checkNS(t, src, "dict")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning for discarded dict.get(), got: %s", diagList(info.Warnings))
	}
}

func TestMustUse_DiscardedMapAndThenChain_Warns(t *testing.T) {
	src := `fn inc(x: int) -> int { return x + 1 }
fn to_opt(x: int) -> Optional[int] { return Some(x) }
` + wrapMain(`and_then(array.map(Some(1), inc), to_opt)`)
	info := checkNS(t, src, "array")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got: %s", diagList(info.Errors))
	}
	if !hasMustUseWarning(info) {
		t.Fatalf("expected a discarded-value warning for a discarded map/and_then chain, got: %s", diagList(info.Warnings))
	}
}
