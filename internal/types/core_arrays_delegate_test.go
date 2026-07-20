package types

import "testing"

// TestCoreArraysContainsIndexOfDelegateWiring proves array.contains/
// array.index_of reach the same flat overload-resolving dispatch the
// string-namespace array form already uses, by confirming a non-comparable
// element type (float[]) still produces the existing domain-check diagnostic
// through the array. spelling -- i.e. Fix #1 didn't bypass checkBuiltinNamed's
// argument-domain checks for either builtin.
func TestCoreArraysContainsIndexOfDelegateWiring(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "contains",
			src:  `fn main() -> int { let xs: float[] = [1.0, 2.0]; let b: bool = array.contains(xs, 1.0); return 0 }`,
			want: "contains on an array is defined only for comparable element types int/bool/string/enum, got [float]",
		},
		{
			name: "index_of",
			src:  `fn main() -> int { let xs: float[] = [1.0, 2.0]; let o: Optional[int] = array.index_of(xs, 1.0); return 0 }`,
			want: "index_of on an array is defined only for comparable element types int/bool/string/enum, got [float]",
		},
	} {
		info := checkArraysProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, errMsgs(info))
		}
	}
}
