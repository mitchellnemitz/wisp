package format

import (
	"strings"
	"testing"
)

func fmtOK(t *testing.T, src string) string {
	t.Helper()
	out, err := Format(src, "test.wisp")
	if err != nil {
		t.Fatalf("Format(%q): %v", src, err)
	}
	return out
}

func TestFormatModuleSurface(t *testing.T) {
	src := `import "owner/repo" as r
include "./lib/util.wisp" as util
include "./other.wisp"

export struct Point { x: int, y: int }

export fn make() -> Point {
return Point { x: 1, y: 2 }
}

fn main() -> int {
let v: r.Value = r.Value { n: util.bump(1) }
return v.n
}
`
	got := fmtOK(t, src)
	for _, want := range []string{
		`import "owner/repo" as r`,
		`include "./lib/util.wisp" as util`,
		`include "./other.wisp"`,
		`export struct Point { x: int, y: int }`,
		`export fn make() -> Point {`,
		`let v: r.Value = r.Value { n: util.bump(1) }`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatted output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestFormatModuleIdempotent(t *testing.T) {
	src := `import "a/b" as b
export fn f(p: b.T) -> b.T { return p }
fn main() -> int { return 0 }
`
	once := fmtOK(t, src)
	twice := fmtOK(t, once)
	if once != twice {
		t.Errorf("not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

// TestImportAdjacencyCollapsesBlank asserts adjacent import/include directives
// with no interleaved comment print with zero blank lines between them
// (acceptance criterion 8, exact full-output match).
func TestImportAdjacencyCollapsesBlank(t *testing.T) {
	src := `import "a" as a
import "b" as b
include "./c.wisp"

fn main() -> int { return 0 }
`
	want := "import \"a\" as a\n" +
		"import \"b\" as b\n" +
		"include \"./c.wisp\"\n" +
		"\n" +
		"fn main() -> int {\n" +
		"    return 0\n" +
		"}\n"
	got := fmtOK(t, src)
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestImportAdjacencyKeepsBlankAroundComment asserts a comment sitting between
// two adjacent directives suppresses the collapse and the blank line is kept
// before the comment (acceptance criterion 9, exact full-output match).
func TestImportAdjacencyKeepsBlankAroundComment(t *testing.T) {
	src := `import "a" as a
// separator
import "b" as b
`
	want := "import \"a\" as a\n" +
		"\n" +
		"// separator\n" +
		"import \"b\" as b\n"
	got := fmtOK(t, src)
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestImportAdjacencyIdempotent asserts format(format(x)) == format(x) for both
// the collapsed and comment-preserved adjacency shapes (acceptance criterion 10).
func TestImportAdjacencyIdempotent(t *testing.T) {
	cases := []string{
		`import "a" as a
import "b" as b
include "./c.wisp"

fn main() -> int { return 0 }
`,
		`import "a" as a
// separator
import "b" as b
`,
	}
	for _, src := range cases {
		once := fmtOK(t, src)
		twice := fmtOK(t, once)
		if once != twice {
			t.Errorf("not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
		}
	}
}

// TestImportConstAdjacencyKeepsBlank is the negative control (acceptance
// criterion 13): a directive next to a non-directive (const) keeps its blank
// line, proving isDirective is false for Consts.
func TestImportConstAdjacencyKeepsBlank(t *testing.T) {
	src := `import "a" as a
const X: int = 1
`
	want := "import \"a\" as a\n" +
		"\n" +
		"const X: int = 1\n"
	got := fmtOK(t, src)
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}
