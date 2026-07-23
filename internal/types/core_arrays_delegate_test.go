package types

import "testing"

// TestCoreArraysContainsIndexOfDelegateWiring proves array.contains/
// array.index_of reach the same flat overload-resolving dispatch the
// string-namespace array form already uses, by confirming a non-comparable
// element type (a struct array) still produces the existing domain-check
// diagnostic through the array. spelling -- i.e. Fix #1 didn't bypass
// checkBuiltinNamed's argument-domain checks for either builtin. float is
// admitted at this gate (uniform scalar comparability), so a struct is used
// here instead as the still-rejected non-comparable element type.
func TestCoreArraysContainsIndexOfDelegateWiring(t *testing.T) {
	const structDecl = `struct P { x: int }
`
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "contains",
			src:  structDecl + `fn main() -> int { let xs: P[] = [P{x: 1}]; let b: bool = array.contains(xs, P{x: 1}); return 0 }`,
			want: "contains on an array is defined only for comparable element types int/bool/string/float/enum, got [P]",
		},
		{
			name: "index_of",
			src:  structDecl + `fn main() -> int { let xs: P[] = [P{x: 1}]; let o: Optional[int] = array.index_of(xs, P{x: 1}); return 0 }`,
			want: "index_of on an array is defined only for comparable element types int/bool/string/float/enum, got [P]",
		},
	} {
		info := checkArraysProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, errMsgs(info))
		}
	}
}
