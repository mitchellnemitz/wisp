package format

import "testing"

// TestOperandParenthesization pins the exact formatted output of the
// precedence-sensitive parenthesization paths (unary operand, binary left
// operand, binary right operand). It is an exact-output guard: it fails on any
// byte drift in how operands are parenthesized.
func TestOperandParenthesization(t *testing.T) {
	cases := []struct{ name, expr string }{
		{"binary_left_needs_parens", "(1 + 2) * 3"},
		{"binary_right_no_parens", "1 + 2 * 3"},
		{"unary_operand_parenthesized", "!(a > b) && b > 0"},
		{"unary_operand_plus_binary_left", "-a + b"},
		{"binary_right_needs_parens", "a - (b - 1)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "fn main() -> int {\n  let x: int = " + c.expr + "\n  return 0\n}"
			want := "fn main() -> int {\n    let x: int = " + c.expr + "\n    return 0\n}\n"
			got := mustFormat(t, src)
			if got != want {
				t.Fatalf("parenthesization drift:\n--got--\n%s\n--want--\n%s", got, want)
			}
		})
	}
}
