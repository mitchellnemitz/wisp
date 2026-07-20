package types

import "testing"

// set_stdin(content: string) -> void: replace fd 0 with content's exact bytes
// (Gap 1 design). Fixed-signature builtin, generic checkBuiltinCall path.

func TestSetStdin_OK(t *testing.T) {
	expectOK(t, wrapMain(`set_stdin("x")`))
}

func TestSetStdin_RequiresArg(t *testing.T) {
	expectErr(t, wrapMain(`set_stdin()`), "set_stdin")
}

func TestSetStdin_RejectsExtraArg(t *testing.T) {
	expectErr(t, wrapMain(`set_stdin("a", "b")`), "set_stdin")
}

func TestSetStdin_ContentMustBeString(t *testing.T) {
	expectErr(t, wrapMain(`set_stdin(1)`), "set_stdin")
}

func TestSetStdin_VoidNotAValue(t *testing.T) {
	expectErr(t, wrapMain(`let x: string = set_stdin("y")`), "void")
}

var setStdinNames = []string{"set_stdin"}

func TestSetStdin_ReservedNames_Fn(t *testing.T) {
	for _, name := range setStdinNames {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectErr(t, src, name)
	}
}

func TestSetStdin_ReservedNames_Let(t *testing.T) {
	for _, name := range setStdinNames {
		expectErr(t, wrapMain("let "+name+": int = 0"), "reserved")
	}
}

func TestSetStdin_ReservedNames_Param(t *testing.T) {
	for _, name := range setStdinNames {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectErr(t, src, name)
	}
}
