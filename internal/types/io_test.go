package types

import "testing"

// M7 (I/O): type-checker coverage for env, has_env, read_file, write_file,
// append_file, run, exit. Positive cases assert the result type flows; negative
// cases assert the located compile error.

// --- well-typed results ---

func TestM7_Exit_OK(t *testing.T) {
	expectOK(t, wrapMain(`exit(2)`))
}

// --- argument typing ---

func TestM7_Exit_CodeMustBeInt(t *testing.T) {
	expectErr(t, wrapMain(`exit("2")`), "exit")
}

// --- reserved names: only stays-flat exit remains reserved ---
// env, has_env, read_file, write_file, append_file, run are now removable (they
// have a module home) and are freed for reuse.

func TestM7_ReservedNames_Fn(t *testing.T) {
	for _, name := range []string{"exit"} {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectErr(t, src, name)
	}
}

func TestM7_ReservedNames_Let(t *testing.T) {
	for _, name := range []string{"exit"} {
		expectErr(t, wrapMain("let "+name+": int = 0"), "reserved")
	}
}

func TestM7_ReservedNames_Param(t *testing.T) {
	for _, name := range []string{"exit"} {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectErr(t, src, name)
	}
}

// --- exit inside try/catch/finally is a compile error (lexical) ---

func TestM7_Exit_InTryBody_Error(t *testing.T) {
	expectErr(t, wrapMain(`try {
exit(1)
} catch (e) {
print("c")
}`), "exit")
}

func TestM7_Exit_InCatchBody_Error(t *testing.T) {
	expectErr(t, wrapMain(`try {
print("t")
} catch (e) {
exit(1)
}`), "exit")
}

func TestM7_Exit_InFinallyBody_Error(t *testing.T) {
	expectErr(t, wrapMain(`try {
print("t")
} catch (e) {
print("c")
} finally {
exit(1)
}`), "exit")
}

// exit is fine outside a try, including in a function called from a try (the
// rule is lexical: only a syntactic exit inside the try body is rejected).
func TestM7_Exit_OutsideTry_OK(t *testing.T) {
	expectOK(t, `fn doExit() -> void { exit(3) }
fn main() -> int {
try {
print("t")
} catch (e) {
print("c")
}
return 0
}`)
}

// --- R3-io-tail: read_line, read_stdin, change_dir, run_full ---

func TestR3_ReadLine_ResultIsOptionalString(t *testing.T) {
	expectOK(t, wrapMain(`let r: Optional[string] = read_line()`))
}

func TestR3_ReadLine_NoArgs(t *testing.T) {
	expectErr(t, wrapMain(`read_line("x")`), "read_line")
}

func TestR3_ReadStdin_ResultIsString(t *testing.T) {
	expectOK(t, wrapMain(`let s: string = read_stdin()`))
}

func TestR3_ReadStdin_NoArgs(t *testing.T) {
	expectErr(t, wrapMain(`read_stdin("x")`), "read_stdin")
}

func TestR3_RunResult_FieldAccess_OK(t *testing.T) {
	// RunResult field access. The value is taken as a parameter since the bare
	// run_full spelling is gone (now process.run_full; see core_process_test.go).
	expectOK(t, `fn fields(r: RunResult) -> int {
  let o: string = r.stdout
  let e: string = r.stderr
  let c: int = r.code
  return c
}
fn main() -> int { return 0 }`)
}

func TestR3_RunResult_UnknownField_Error(t *testing.T) {
	// The handle is threaded via a parameter (no bare run_full, now process.run_full).
	expectErr(t, `fn f(r: RunResult) -> int {
  let m: string = r.message
  return 0
}
fn main() -> int { return 0 }`, "RunResult")
}

func TestR3_RunResult_NoEquality(t *testing.T) {
	expectErr(t, `fn f(r: RunResult) -> int {
  let ok: bool = r == r
  return 0
}
fn main() -> int { return 0 }`, "")
}

func TestR3_RunResult_NoStringConv(t *testing.T) {
	expectErr(t, `fn f(r: RunResult) -> int {
  let s: string = to_string(r)
  return 0
}
fn main() -> int { return 0 }`, "")
}

func TestR3_RunResult_AsUserFuncParam(t *testing.T) {
	// RunResult passes through user-function parameters. The handle is threaded
	// via parameters (no bare run_full, which is now process.run_full).
	expectOK(t, `fn describe(r: RunResult) -> string { return r.stdout }
fn relay(r: RunResult) -> string { return describe(r) }
fn main() -> int { return 0 }`)
}

func TestR3_ReservedNames_Builtins(t *testing.T) {
	// change_dir and run_full are now removable (they have a module home) and are
	// freed for reuse; only the stays-flat io builtins remain reserved.
	for _, name := range []string{"read_line", "read_stdin"} {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectErr(t, src, name)
	}
}

func TestR3_ReservedNames_RunResult_Type(t *testing.T) {
	expectErr(t, wrapMain("let RunResult: int = 0"), "reserved")
}

// --- read_secret ---

func TestReadSecret_Checks(t *testing.T) {
	expectOK(t, `fn main() -> int { let r: Optional[string] = read_secret("pw: "); return 0 }`)
	expectOK(t, `fn main() -> int { let r: Optional[string] = read_secret("pw: "); print("after"); return 0 }`) // not a terminator

	expectErr(t, `fn main() -> int { let r: Optional[string] = read_secret(); return 0 }`, "expects")
	expectErr(t, `fn main() -> int { let r: Optional[string] = read_secret("a","b"); return 0 }`, "expects")
	expectErr(t, `fn main() -> int { let r: Optional[string] = read_secret(1); return 0 }`, "has type")

	expectErr(t, `fn read_secret() -> int { return 0 } fn main() -> int { return 0 }`, "")
	expectErr(t, `fn main() -> int { let read_secret: int = 1; return 0 }`, "")
}

func TestReadSecret_NotReferenceAllowed(t *testing.T) {
	expectErr(t, `fn main()->int{ let f: fn(string)->Optional[string] = read_secret; return 0 }`, "funcref-shaped")
}

func TestR3_RunResult_InTypeNames(t *testing.T) {
	found := false
	for _, n := range TypeNames() {
		if n == "RunResult" {
			found = true
			break
		}
	}
	if !found {
		t.Error("TypeNames() missing RunResult")
	}
}
