package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// hasCheckErr parses+checks src and reports whether any checker error message
// contains sub.
func hasCheckErr(t *testing.T, src, sub string) bool {
	t.Helper()
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := types.Check(prog)
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, sub) {
			return true
		}
	}
	return false
}

const ctxEmptyList = "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"

// SC-005: a context-inferred call compiles to byte-identical shell as the
// explicit-type-argument form of the same program.
func TestContextInferByteIdenticalToExplicit(t *testing.T) {
	inferred := compile(t, ctxEmptyList+
		"fn main() -> int {\n let xs: int[] = empty_list()\n print(to_string(length(xs)))\n return 0\n}")
	explicit := compile(t, ctxEmptyList+
		"fn main() -> int {\n let xs: int[] = empty_list[int]()\n print(to_string(length(xs)))\n return 0\n}")
	if !bytes.Equal(inferred, explicit) {
		t.Fatalf("context-inferred output differs from explicit-type-argument output\n--- inferred ---\n%s\n--- explicit ---\n%s", inferred, explicit)
	}
}

// SC-005 per-position byte-identity: the return, call-argument, assignment,
// array-element, and dict-value positions each compile byte-identically to their
// explicit-type-argument form. Together with TestContextInferByteIdenticalToExplicit
// (the let/final binding position) this directly covers byte-identity for every
// SC-001..SC-004 position; the golden fixture (Task 5) additionally proves the
// dict-value and other positions identical at runtime across all four shells.
func TestContextInferByteIdenticalPerPosition(t *testing.T) {
	cases := []struct {
		name, inferred, explicit string
	}{
		{
			"return",
			ctxEmptyList + "fn make() -> int[] {\n return empty_list()\n}\n" + "fn main() -> int {\n let xs: int[] = make()\n print(to_string(length(xs)))\n return 0\n}",
			ctxEmptyList + "fn make() -> int[] {\n return empty_list[int]()\n}\n" + "fn main() -> int {\n let xs: int[] = make()\n print(to_string(length(xs)))\n return 0\n}",
		},
		{
			"callarg",
			ctxEmptyList + "fn takes(xs: int[]) -> int {\n return length(xs)\n}\n" + "fn main() -> int {\n print(to_string(takes(empty_list())))\n return 0\n}",
			ctxEmptyList + "fn takes(xs: int[]) -> int {\n return length(xs)\n}\n" + "fn main() -> int {\n print(to_string(takes(empty_list[int]())))\n return 0\n}",
		},
		{
			"assign",
			ctxEmptyList + "fn main() -> int {\n let xs: int[] = [1]\n xs = empty_list()\n print(to_string(length(xs)))\n return 0\n}",
			ctxEmptyList + "fn main() -> int {\n let xs: int[] = [1]\n xs = empty_list[int]()\n print(to_string(length(xs)))\n return 0\n}",
		},
		{
			"array-elem",
			ctxEmptyList + "fn main() -> int {\n let grid: int[][] = [empty_list()]\n print(to_string(length(grid)))\n return 0\n}",
			ctxEmptyList + "fn main() -> int {\n let grid: int[][] = [empty_list[int]()]\n print(to_string(length(grid)))\n return 0\n}",
		},
		{
			"dict-value",
			ctxEmptyList + "fn main() -> int {\n let m: {string: int[]} = {\"k\": empty_list()}\n print(to_string(length(m[\"k\"])))\n return 0\n}",
			ctxEmptyList + "fn main() -> int {\n let m: {string: int[]} = {\"k\": empty_list[int]()}\n print(to_string(length(m[\"k\"])))\n return 0\n}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !bytes.Equal(compile(t, tc.inferred), compile(t, tc.explicit)) {
				t.Fatalf("%s: context-inferred output differs from explicit-type-argument output", tc.name)
			}
		})
	}
}

// SC-006: a numeric-bounded return-only param pinned by context is monomorphized
// for the pinned type, byte-identical to the explicit form; and int vs float
// produce DIFFERENT output (proving specialization actually occurred).
const ctxZeros = "fn zeros[T: numeric](n: int) -> T[] {\n let xs: T[] = []\n return xs\n}\n"

func TestContextInferNumericMonoByteIdentical(t *testing.T) {
	inferred := compile(t, ctxZeros+
		"fn main() -> int {\n let xs: float[] = zeros(3)\n print(to_string(length(xs)))\n return 0\n}")
	explicit := compile(t, ctxZeros+
		"fn main() -> int {\n let xs: float[] = zeros[float](3)\n print(to_string(length(xs)))\n return 0\n}")
	if !bytes.Equal(inferred, explicit) {
		t.Fatalf("context-pinned numeric mono differs from explicit form\n--- inferred ---\n%s\n--- explicit ---\n%s", inferred, explicit)
	}
	floatOut := compile(t, ctxZeros+
		"fn main() -> int {\n let xs: float[] = zeros(3)\n return 0\n}")
	intOut := compile(t, ctxZeros+
		"fn main() -> int {\n let xs: int[] = zeros(3)\n return 0\n}")
	if bytes.Equal(floatOut, intOut) {
		t.Fatalf("numeric mono produced identical output for int and float bindings; specialization did not occur")
	}
}

// SC-006 violation path: a context-pinned type that violates the numeric bound is
// rejected with the existing bound-violation error.
func TestContextInferNumericBoundViolation(t *testing.T) {
	if !hasCheckErr(t, ctxZeros+
		"fn main() -> int {\n let xs: string[] = zeros(3)\n return 0\n}", "does not satisfy numeric") {
		t.Fatalf("expected a numeric bound-violation error")
	}
}

// SC-014: a comparable-bounded return-only param pinned by context compiles
// byte-identically to the explicit form (comparable is type-erased).
const ctxEmptyOf = "fn empty_of[T: comparable]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"

func TestContextInferComparableByteIdentical(t *testing.T) {
	inferred := compile(t, ctxEmptyOf+
		"fn main() -> int {\n let xs: int[] = empty_of()\n print(to_string(length(xs)))\n return 0\n}")
	explicit := compile(t, ctxEmptyOf+
		"fn main() -> int {\n let xs: int[] = empty_of[int]()\n print(to_string(length(xs)))\n return 0\n}")
	if !bytes.Equal(inferred, explicit) {
		t.Fatalf("context-pinned comparable differs from explicit form\n--- inferred ---\n%s\n--- explicit ---\n%s", inferred, explicit)
	}
}

// SC-014 violation path: a context-pinned non-comparable type is rejected with the
// existing comparable bound-violation error.
func TestContextInferComparableBoundViolation(t *testing.T) {
	if !hasCheckErr(t, "struct S { x: int }\n"+ctxEmptyOf+
		"fn main() -> int {\n let xs: S[] = empty_of()\n return 0\n}", "does not satisfy comparable") {
		t.Fatalf("expected a comparable bound-violation error")
	}
}
