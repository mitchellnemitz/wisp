package format

import (
	"strings"
	"testing"
)

func mustFormat(t *testing.T, src string) string {
	t.Helper()
	out, err := Format(src, "t.wisp")
	if err != nil {
		t.Fatalf("Format(%q): %v", src, err)
	}
	return out
}

// TestCanonicalExact pins the canonical style for a representative program:
// 4-space indent, one space around binary ops and after , and :, no inner
// bracket spacing, `{` on the construct line, `}` own line, one blank line
// between top-level decls, single trailing newline.
func TestCanonicalExact(t *testing.T) {
	src := "struct Point{x:int,y:int}\n" +
		"fn add(a:int,b:int)->int{return a+b}\n" +
		"fn main()->int{\n" +
		"let p:Point=Point{x:1,y:2}\n" +
		"let s:int=add(p.x,p.y)\n" +
		"if(s>0){print(\"pos\")}else{print(\"neg\")}\n" +
		"return 0\n" +
		"}\n"
	want := "struct Point { x: int, y: int }\n" +
		"\n" +
		"fn add(a: int, b: int) -> int {\n" +
		"    return a + b\n" +
		"}\n" +
		"\n" +
		"fn main() -> int {\n" +
		"    let p: Point = Point { x: 1, y: 2 }\n" +
		"    let s: int = add(p.x, p.y)\n" +
		"    if (s > 0) {\n" +
		"        print(\"pos\")\n" +
		"    } else {\n" +
		"        print(\"neg\")\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("canonical mismatch:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestFuncTypeParamsPreserved pins that formatting keeps a function's generic
// type-parameter list and bounds (regression: fmt dropped `T[]`/`[T: bound]`,
// producing a program that no longer compiled).
func TestFuncTypeParamsPreserved(t *testing.T) {
	cases := map[string]string{
		"fn id[T](x:T)->T{return x}\nfn main()->int{return 0}\n":                      "fn id[T](x: T) -> T {",
		"fn eq[T:comparable](a:T,b:T)->bool{return a==b}\nfn main()->int{return 0}\n": "fn eq[T: comparable](a: T, b: T) -> bool {",
		"fn add[T:numeric](a:T,b:T)->T{return a+b}\nfn main()->int{return 0}\n":       "fn add[T: numeric](a: T, b: T) -> T {",
	}
	for src, wantLine := range cases {
		got := mustFormat(t, src)
		if !strings.Contains(got, wantLine) {
			t.Errorf("fmt dropped/garbled type params:\n got:\n%s\n want a line:\n%s", got, wantLine)
		}
		// And the formatted output must itself be valid + idempotent.
		if mustFormat(t, got) != got {
			t.Errorf("not idempotent for %q", src)
		}
	}
}

// corpus exercises every construct for the idempotence and structural checks.
var corpus = []string{
	// minimal
	"fn main() -> int { return 0 }",
	// struct + fields + nested types
	"struct Pair { a: int, b: string[] }\nfn main() -> int { return 0 }",
	// all the statement forms
	`fn main() -> int {
  let x: int = 1
  x = x + 2
  let xs: int[] = [1, 2, 3]
  xs[0] = 9
  let m: {string: int} = { "a": 1, "b": 2 }
  m["c"] = 3
  while (x < 10) {
    x = x + 1
    if (x == 5) {
      break
    } else if (x == 3) {
      continue
    }
  }
  for (let i: int = 0; i < 3; i = i + 1) {
    print("${i}")
  }
  for (v in xs) {
    print("${v}")
  }
  switch (x) {
    case 1, 2 {
      print("low")
    }
    default {
      print("hi")
    }
  }
  return 0
}`,
	// try / catch / finally + throw
	`fn risky() -> int {
  try {
    print("body")
  } catch (e) {
    print(e.message)
  } finally {
    print("cleanup")
  }
  return 0
}
fn main() -> int {
  return risky()
}`,
	// operators + precedence + parens
	`fn main() -> int {
  let a: int = (1 + 2) * 3
  let b: int = 1 + 2 * 3
  let c: bool = !(a > b) && b > 0
  let d: int = -a + b
  let e: int = a - (b - 1)
  print("${a + b + c + d + e}")
  return 0
}`,
	// function references, higher order, defaults
	`fn dbl(x: int) -> int {
  return x * 2
}
fn greet(name: string = "world") -> string {
  return "hi ${name}"
}
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = map(xs, dbl)
  print(greet())
  print("${length(ys)}")
  return 0
}`,
	// string escapes + interpolation
	`fn main() -> int {
  print("tab\there and a quote \" and dollar \$ and backslash \\")
  let n: int = 3
  print("n is ${n} and n+1 is ${n + 1}")
  return 0
}`,
	// fn-type annotation
	`fn apply(f: fn(int, int) -> int, a: int, b: int) -> int {
  return f(a, b)
}
fn add(a: int, b: int) -> int {
  return a + b
}
fn main() -> int {
  print("${apply(add, 2, 3)}")
  return 0
}`,
	// empty struct/array/dict literals + empty blocks
	`fn noop() -> void {
}
fn main() -> int {
  let xs: int[] = []
  let m: {string: int} = {}
  noop()
  print("${length(xs)}")
  return 0
}`,
}

func TestIdempotent(t *testing.T) {
	for i, src := range corpus {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Fatalf("corpus[%d] not idempotent:\n--once--\n%s\n--twice--\n%s", i, once, twice)
		}
	}
}

func TestStructuralInvariants(t *testing.T) {
	for i, src := range corpus {
		out := mustFormat(t, src)
		if strings.Contains(out, "\t") {
			t.Fatalf("corpus[%d] output contains a tab", i)
		}
		if strings.HasPrefix(out, "\n") {
			t.Fatalf("corpus[%d] output has a leading blank line", i)
		}
		if !strings.HasSuffix(out, "\n") {
			t.Fatalf("corpus[%d] output missing trailing newline", i)
		}
		if strings.HasSuffix(out, "\n\n") {
			t.Fatalf("corpus[%d] output has more than one trailing newline", i)
		}
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if line != strings.TrimRight(line, " \t") {
				t.Fatalf("corpus[%d] has trailing whitespace on line %q", i, line)
			}
			// No statement is terminated by ';' (the only ';' allowed is inside a
			// C-style for header). A formatted line must never END with ';'.
			if strings.HasSuffix(line, ";") {
				t.Fatalf("corpus[%d] has a ';'-terminated statement line %q", i, line)
			}
		}
	}
}

func TestParseErrorNoOutput(t *testing.T) {
	out, err := Format("fn main( -> int { return 0 }", "bad.wisp")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if out != "" {
		t.Fatalf("expected no output on parse error, got %q", out)
	}
	// the error must be located (file:line:col)
	if !strings.Contains(err.Error(), "bad.wisp:") {
		t.Fatalf("error not located: %v", err)
	}
}

// --- comment round-trip ---

func TestCommentFullLine(t *testing.T) {
	src := "// a leading comment\nfn main() -> int {\n  // inside\n  return 0\n}\n"
	out := mustFormat(t, src)
	want := "// a leading comment\n" +
		"fn main() -> int {\n" +
		"    // inside\n" +
		"    return 0\n" +
		"}\n"
	if out != want {
		t.Fatalf("full-line comment:\n--got--\n%s\n--want--\n%s", out, want)
	}
	if out != mustFormat(t, out) {
		t.Fatalf("comment format not idempotent")
	}
}

func TestCommentTrailing(t *testing.T) {
	src := "fn main() -> int {\n  let x: int = 1 // the one\n  return x // done\n}\n"
	out := mustFormat(t, src)
	want := "fn main() -> int {\n" +
		"    let x: int = 1 // the one\n" +
		"    return x // done\n" +
		"}\n"
	if out != want {
		t.Fatalf("trailing comment:\n--got--\n%s\n--want--\n%s", out, want)
	}
	if out != mustFormat(t, out) {
		t.Fatalf("trailing comment format not idempotent")
	}
}

func TestCommentStackedOrder(t *testing.T) {
	src := "fn main() -> int {\n  // first\n  // second\n  // third\n  return 0\n}\n"
	out := mustFormat(t, src)
	want := "fn main() -> int {\n" +
		"    // first\n" +
		"    // second\n" +
		"    // third\n" +
		"    return 0\n" +
		"}\n"
	if out != want {
		t.Fatalf("stacked comments:\n--got--\n%s\n--want--\n%s", out, want)
	}
	if out != mustFormat(t, out) {
		t.Fatalf("stacked comment format not idempotent")
	}
}

func TestCommentMixedRoundTripIdempotent(t *testing.T) {
	src := "// file header\n" +
		"// continued\n" +
		"struct P { x: int } // a point\n" +
		"fn main() -> int { // entry\n" +
		"  let p: P = P { x: 1 } // make it\n" +
		"  // about to return\n" +
		"  return p.x\n" +
		"}\n" +
		"// trailing file comment\n"
	out := mustFormat(t, src)
	if out != mustFormat(t, out) {
		t.Fatalf("mixed comments not idempotent:\n--once--\n%s\n--twice--\n%s", out, mustFormat(t, out))
	}
	// every comment text must survive
	for _, want := range []string{
		"// file header", "// continued", "// a point", "// entry",
		"// make it", "// about to return", "// trailing file comment",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("comment %q lost in round-trip:\n%s", want, out)
		}
	}
}

// TestConstFinalRoundTrip: const and final statements round-trip unchanged
// through the formatter (idempotent, exact shape preserved).
func TestConstFinalRoundTrip(t *testing.T) {
	cases := []string{
		"fn main() -> int {\n    const x: int = 42\n    return x\n}\n",
		"fn main() -> int {\n    final y: int = 1\n    return y\n}\n",
		"const LIMIT: int = 100\n\nfn main() -> int {\n    return 0\n}\n",
	}
	for _, src := range cases {
		out := mustFormat(t, src)
		if out != src {
			t.Errorf("round-trip mismatch for %q:\ngot:  %q\nwant: %q", src, out, src)
		}
		if out != mustFormat(t, out) {
			t.Errorf("not idempotent for %q", src)
		}
	}
}

// TestFormatTupleIdempotent: formatting a tuple literal / tuple type / tuple
// access is idempotent (a second Format pass is a no-op).
func TestFormatTupleIdempotent(t *testing.T) {
	cases := []string{
		"fn main() -> int {\nlet t: (int, string) = (1, \"a\")\nreturn 0\n}\n",
		"fn main() -> int {\nlet t: (int, int, int) = (1, 2, 3)\nreturn 0\n}\n",
		"fn main() -> int {\nlet t: (int, (bool, int)) = (1, (true, 2))\nreturn 0\n}\n",
		// tuple access
		"fn main() -> int {\nlet t: (int, string) = (1, \"a\")\nlet x: int = t[0]\nreturn 0\n}\n",
		// nested access t[0][1]
		"fn main() -> int {\nlet t: ((int, bool), string) = ((3, true), \"x\")\nlet x: int = t[0][1]\nreturn 0\n}\n",
		// tuple type in param and return position
		"fn f(t: (int, bool)) -> (int, string) { return (t[0], \"x\") }\nfn main() -> int { return 0 }\n",
	}
	for _, src := range cases {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Errorf("not idempotent for %q:\n once=%q\n twice=%q", src, once, twice)
		}
	}
}

// TestFormatTupleExactShape: pin the canonical rendering -- one space after each
// comma, no trailing comma, in both the literal and the type annotation. Assert
// the exact formatted lines, not merely idempotency.
func TestFormatTupleExactShape(t *testing.T) {
	// Literal: input with a trailing comma and tight spacing must render (1, 2).
	out := mustFormat(t, "fn main() -> int {\nlet t: (int,int) = (1,2,)\nreturn 0\n}\n")
	if !strings.Contains(out, "let t: (int, int) = (1, 2)\n") {
		t.Errorf("literal/type shape wrong; got:\n%s", out)
	}
	if strings.Contains(out, "(1, 2,)") || strings.Contains(out, "(1,2)") {
		t.Errorf("expected canonical `(1, 2)` with no trailing comma; got:\n%s", out)
	}
	// Type annotation with tight spacing must render (int, string).
	outT := mustFormat(t, "fn f() -> (int,string) { return f() }\nfn main() -> int { return 0 }\n")
	if !strings.Contains(outT, "-> (int, string)") {
		t.Errorf("tuple type shape wrong; expected `(int, string)`, got:\n%s", outT)
	}
}
