package types

import "testing"

// TestRemovedBuiltins_IntOrFloatOrGetOrEnvOrUndeclared is the SC-007 negative
// test: int_or/float_or/get_or/env_or are gone entirely (not moved to a
// module -- unwrap_or is the single fallback now), so a bare call to any of
// them must fail as an undeclared function, exactly like a name that was
// never a builtin. This is the one tracked file exempt from SC-014's
// zero-references sweep (Task 8 Step 1 excludes this exact path).
func TestRemovedBuiltins_IntOrFloatOrGetOrEnvOrUndeclared(t *testing.T) {
	for name, call := range map[string]string{
		"int_or":   `let i: int = int_or("42", -1)`,
		"float_or": `let f: float = float_or("3.14", -1.0)`,
		"get_or":   `let d: {string: int} = {}; let v: int = get_or(d, "a", -1)`,
		"env_or":   `let s: string = env_or("PATH", "FB")`,
	} {
		t.Run(name, func(t *testing.T) {
			expectErr(t, wrapMain(call), "undeclared function")
		})
	}
}
