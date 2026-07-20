package format

import (
	"strings"
	"testing"
)

// wrapMain wraps a body line into a main function for formatting tests.
func wrapMain(body string) string {
	return "fn main() -> int {\n" + body + "\n    return 0\n}\n"
}

// TestMultilineArrayPreserved: an array the user wrote across multiple lines is
// rendered one element per line, indented one level deeper, each element with a
// trailing comma (including the last), closer on its own line at the construct
// indent.
func TestMultilineArrayPreserved(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let xs: int[] = [\n" +
		"        1,\n" +
		"        2,\n" +
		"        3\n" +
		"    ]\n" +
		"    return 0\n}\n"
	want := "fn main() -> int {\n" +
		"    let xs: int[] = [\n" +
		"        1,\n" +
		"        2,\n" +
		"        3,\n" +
		"    ]\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("multi-line array:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestMultilineDictPreserved: a multi-line dict renders one entry per line with
// a trailing comma and the closer on its own line.
func TestMultilineDictPreserved(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let d: {string: int} = {\n" +
		"        \"a\": 1,\n" +
		"        \"b\": 2\n" +
		"    }\n" +
		"    return 0\n}\n"
	want := "fn main() -> int {\n" +
		"    let d: {string: int} = {\n" +
		"        \"a\": 1,\n" +
		"        \"b\": 2,\n" +
		"    }\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("multi-line dict:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestMultilineStructLitPreserved: a multi-line struct literal renders one field
// per line with a trailing comma.
func TestMultilineStructLitPreserved(t *testing.T) {
	src := "struct Point { x: int, y: int }\n" +
		"fn main() -> int {\n" +
		"    let p: Point = Point {\n" +
		"        x: 1,\n" +
		"        y: 2\n" +
		"    }\n" +
		"    return 0\n}\n"
	want := "struct Point { x: int, y: int }\n\n" +
		"fn main() -> int {\n" +
		"    let p: Point = Point {\n" +
		"        x: 1,\n" +
		"        y: 2,\n" +
		"    }\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("multi-line struct lit:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestMultilineStructDeclPreserved: a multi-line struct declaration renders one
// field per line at depth 1 with a trailing comma, closer at depth 0.
func TestMultilineStructDeclPreserved(t *testing.T) {
	src := "struct Point {\n" +
		"    x: int,\n" +
		"    y: int\n" +
		"}\n" +
		"fn main() -> int { return 0 }\n"
	want := "struct Point {\n" +
		"    x: int,\n" +
		"    y: int,\n" +
		"}\n\n" +
		"fn main() -> int {\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("multi-line struct decl:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestSingleLineNotReflowed: single-line literals stay single-line with NO
// trailing comma (byte-identical to today's canonical form).
func TestSingleLineNotReflowed(t *testing.T) {
	src := "struct Point { x: int, y: int }\n" +
		"fn main() -> int {\n" +
		"    let xs: int[] = [1, 2, 3]\n" +
		"    let d: {string: int} = { \"a\": 1, \"b\": 2 }\n" +
		"    let p: Point = Point { x: 1, y: 2 }\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	want := "struct Point { x: int, y: int }\n\n" +
		"fn main() -> int {\n" +
		"    let xs: int[] = [1, 2, 3]\n" +
		"    let d: {string: int} = { \"a\": 1, \"b\": 2 }\n" +
		"    let p: Point = Point { x: 1, y: 2 }\n" +
		"    return 0\n}\n"
	if got != want {
		t.Fatalf("single-line reflowed:\n--got--\n%s\n--want--\n%s", got, want)
	}
	for _, bad := range []string{",]", ",}", ", ]", ", }"} {
		if strings.Contains(got, bad) {
			t.Fatalf("single-line gained a trailing comma %q:\n%s", bad, got)
		}
	}
}

// TestSemicolonDictNotReflowed: a one-physical-line dict using `;` separators has
// Multiline==false and must NOT be reflowed (N4: the formatter never introduces
// line breaks the user did not write).
func TestSemicolonDictNotReflowed(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let d: {string: int} = { \"a\": 1; \"b\": 2 }\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if !strings.Contains(got, "let d: {string: int} = { \"a\": 1, \"b\": 2 }\n") {
		t.Fatalf("semicolon dict reflowed (must stay single line):\n%s", got)
	}
}

// TestMultilineNestingComposes: a multi-line array of multi-line dicts indents
// each level one deeper than its parent.
func TestMultilineNestingComposes(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let xs: {string: int}[] = [\n" +
		"        {\n" +
		"            \"a\": 1\n" +
		"        },\n" +
		"        {\n" +
		"            \"b\": 2\n" +
		"        }\n" +
		"    ]\n" +
		"    return 0\n}\n"
	want := "fn main() -> int {\n" +
		"    let xs: {string: int}[] = [\n" +
		"        {\n" +
		"            \"a\": 1,\n" +
		"        },\n" +
		"        {\n" +
		"            \"b\": 2,\n" +
		"        },\n" +
		"    ]\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("nested multi-line:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestMultilineEmptyStaysSingleLine: an empty literal renders single-line even
// when newlines appear inside (R6: render multi-line IFF flag && len>0).
func TestMultilineEmptyStaysSingleLine(t *testing.T) {
	src := "struct E {\n}\n" +
		"fn main() -> int {\n" +
		"    let xs: int[] = [\n]\n" +
		"    let d: {string: int} = {\n}\n" +
		"    let e: E = E {\n}\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	for _, want := range []string{
		"struct E {}\n",
		"let xs: int[] = []\n",
		"let d: {string: int} = {}\n",
		"let e: E = E {}\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("empty literal not single-line, missing %q:\n%s", want, got)
		}
	}
}

// TestMultilineIdempotent: fmt(fmt(x)) == fmt(x) for both single- and multi-line
// forms. Re-parsing multi-line output re-sets Multiline and re-emits identically.
func TestMultilineIdempotent(t *testing.T) {
	cases := []string{
		// multi-line array
		"fn main() -> int {\n    let xs: int[] = [\n        1,\n        2,\n    ]\n    return 0\n}\n",
		// multi-line dict
		"fn main() -> int {\n    let d: {string: int} = {\n        \"a\": 1,\n        \"b\": 2,\n    }\n    return 0\n}\n",
		// multi-line struct lit
		"struct P { x: int }\nfn main() -> int {\n    let p: P = P {\n        x: 1,\n    }\n    return 0\n}\n",
		// multi-line struct decl
		"struct P {\n    x: int,\n    y: int,\n}\nfn main() -> int { return 0 }\n",
		// nested
		"fn main() -> int {\n    let xs: int[][] = [\n        [\n            1,\n        ],\n    ]\n    return 0\n}\n",
		// single-line forms
		"fn main() -> int {\n    let xs: int[] = [1, 2, 3]\n    return 0\n}\n",
		"fn main() -> int {\n    let d: {string: int} = { \"a\": 1, \"b\": 2 }\n    return 0\n}\n",
	}
	for _, src := range cases {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Fatalf("not idempotent for %q:\n--once--\n%s\n--twice--\n%s", src, once, twice)
		}
	}
}

// TestMultilineFullLineCommentNotLost: a full-line comment between two items of a
// multi-line literal must NOT be lost (R8: no-loss required; exact placement is a
// documented limitation, may land at the enclosing block indent).
func TestMultilineFullLineCommentNotLost(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let xs: int[] = [\n" +
		"        1,\n" +
		"        // a note about the second element\n" +
		"        2,\n" +
		"    ]\n" +
		"    return 0\n}\n"
	got := mustFormat(t, src)
	if !strings.Contains(got, "// a note about the second element") {
		t.Fatalf("full-line comment inside multi-line literal was LOST:\n%s", got)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("comment-bearing multi-line literal not idempotent:\n%s", got)
	}
}
