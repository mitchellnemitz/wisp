package format

import (
	"strings"
	"testing"
)

// mainTail is the canonical expanded form of the single-line `fn main` used to
// terminate the enum fixtures (the formatter expands a single-line func body).
const mainTail = "fn main() -> int {\n    return 0\n}\n"

// TestEnumSingleLineRoundTrip: a single-line enum renders on one line with inner
// brace spacing and `, ` separators, like a single-line struct decl.
func TestEnumSingleLineRoundTrip(t *testing.T) {
	src := "enum Color { Red, Green, Blue }\n" +
		"fn main() -> int { return 0 }\n"
	want := "enum Color { Red, Green, Blue }\n\n" + mainTail
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("single-line enum:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestEnumMultilinePreserved: a multi-line enum stays multi-line, one variant per
// line at depth 1 with a trailing comma after every variant, closer at depth 0.
func TestEnumMultilinePreserved(t *testing.T) {
	src := "enum State {\n" +
		"    Idle,\n" +
		"    Running,\n" +
		"    Done\n" +
		"}\n" +
		"fn main() -> int { return 0 }\n"
	want := "enum State {\n" +
		"    Idle,\n" +
		"    Running,\n" +
		"    Done,\n" +
		"}\n\n" + mainTail
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("multi-line enum:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestEnumExplicitValuePreserved: an explicit `= value` is preserved, single-line.
func TestEnumExplicitValueSingleLine(t *testing.T) {
	src := "enum ExitCode { Ok = 0, Fail = 1, Usage = 2 }\n" +
		"fn main() -> int { return 0 }\n"
	want := "enum ExitCode { Ok = 0, Fail = 1, Usage = 2 }\n\n" + mainTail
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("enum explicit value single-line:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestEnumExplicitValueMultiline: explicit values preserved in multi-line layout,
// including a negative value.
func TestEnumExplicitValueMultiline(t *testing.T) {
	src := "enum E {\n" +
		"    A = -1,\n" +
		"    B,\n" +
		"    C = 5\n" +
		"}\n" +
		"fn main() -> int { return 0 }\n"
	want := "enum E {\n" +
		"    A = -1,\n" +
		"    B,\n" +
		"    C = 5,\n" +
		"}\n\n" + mainTail
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("enum explicit value multi-line:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestEnumIdempotent: fmt(fmt(x)) == fmt(x) for both single-line and multi-line
// enum forms.
func TestEnumIdempotent(t *testing.T) {
	for _, src := range []string{
		"enum Color { Red, Green, Blue }\nfn main() -> int { return 0 }\n",
		"enum State {\n    Idle,\n    Running,\n    Done\n}\nfn main() -> int { return 0 }\n",
		"enum ExitCode { Ok = 0, Fail = 1, Usage = 2 }\nfn main() -> int { return 0 }\n",
	} {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
		}
	}
}

// TestEnumDeclarationOrderPreserved: an enum interleaved with a struct and a func
// renders in source order.
func TestEnumDeclarationOrderPreserved(t *testing.T) {
	src := "struct P { x: int }\n" +
		"enum Color { Red, Green }\n" +
		"fn main() -> int { return 0 }\n"
	got := mustFormat(t, src)
	pi := strings.Index(got, "struct P")
	ei := strings.Index(got, "enum Color")
	fi := strings.Index(got, "fn main")
	if !(pi >= 0 && pi < ei && ei < fi) {
		t.Fatalf("declaration order not preserved:\n%s", got)
	}
}

// TestNoEnumUnchanged: a canonical program with no enum is a fixed point of the
// formatter (the enum loop entry must not perturb enum-free output).
func TestNoEnumUnchanged(t *testing.T) {
	src := "struct Point { x: int, y: int }\n\n" + mainTail
	got := mustFormat(t, src)
	if got != src {
		t.Fatalf("no-enum file changed:\n--got--\n%s\n--want--\n%s", got, src)
	}
}
