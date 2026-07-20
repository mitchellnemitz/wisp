package types

import "testing"

// M6 stdlib builtins (split/join/contains/starts_with/ends_with/index_of/repeat/
// abs/min/max/reverse/reduce) plus exec_command/set_env/unset_env/run_input(_full)
// are now removable: they live in the string/math/array/process/env modules.
// Positive member-result coverage lives in the core_strings/core_math/
// core_arrays/core_process/core_env suites; the type/overload/domain negatives
// migrated to core_stdlib_neg_test.go. The bare reserved-name tests are gone:
// those names are freed for reuse under PR C.

// index_of_elem was never promoted to a module member and remains undeclared;
// referencing it is an unknown-function error. This is not removable-builtin
// behavior, so it stays here.
func TestIndexOf_ElemRemoved_Error(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1, 2]
let i: Optional[int] = index_of_elem(xs, 1)`), "index_of_elem")
}

// When an argument is itself unresolved (undeclared), the checker reports THAT
// error; the moved-to-module note does not mask it. Uses a removable builtin call
// only as the carrier for the undeclared argument.
func TestM6_Contains_Arg1Unresolved(t *testing.T) {
	info := check(t, wrapMain(`let b: bool = contains(nope, "x")`))
	if len(info.Errors) == 0 {
		t.Fatal("expected an error")
	}
	found := false
	for _, d := range info.Errors {
		if containsStr(d.Msg, "nope") || containsStr(d.Msg, "undeclared") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an arg-1 (undeclared) error, got %s", diagList(info.Errors))
	}
}

func containsStr(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
