package format

import (
	"strings"
	"testing"
)

// wrapBody wraps statement source in a minimal fn so it parses as a program.
func wrapBody(body string) string {
	return "fn main() -> int {\n" + body + "\nreturn 0\n}\n"
}

// TestTupleBindNotDropped is the no-silent-drop guard: a tuple-destructuring
// `let` must NOT vanish from the formatter output. The statement switch has no
// default arm, so before the formatter case is added the binding is silently
// deleted; this fails until the case exists.
func TestTupleBindNotDropped(t *testing.T) {
	got := mustFormat(t, wrapBody("let (a: int, b: string) = pair()"))
	if !strings.Contains(got, "let (a: int, b: string) = pair()") {
		t.Fatalf("tuple-destructuring let was dropped/garbled:\n%s", got)
	}
}

func TestTupleBindLetExact(t *testing.T) {
	got := mustFormat(t, wrapBody("let (a:int,b:string)=pair()"))
	want := "    let (a: int, b: string) = pair()\n"
	if !strings.Contains(got, want) {
		t.Fatalf("let render mismatch:\n--got--\n%s\n--want line--\n%s", got, want)
	}
}

func TestTupleBindFinalExact(t *testing.T) {
	got := mustFormat(t, wrapBody("final (a:int,b:string)=pair()"))
	want := "    final (a: int, b: string) = pair()\n"
	if !strings.Contains(got, want) {
		t.Fatalf("final render mismatch:\n--got--\n%s\n--want line--\n%s", got, want)
	}
}

func TestTupleBindBlankSlots(t *testing.T) {
	cases := map[string]string{
		"let (_,out:string)=pair()":     "    let (_, out: string) = pair()\n",
		"let (_:int,out:string)=pair()": "    let (_: int, out: string) = pair()\n",
		"let (a:int,_)=pair()":          "    let (a: int, _) = pair()\n",
		"let (_,_)=pair()":              "    let (_, _) = pair()\n",
		"let (a:int,b:int[])=pair()":    "    let (a: int, b: int[]) = pair()\n",
	}
	for src, want := range cases {
		got := mustFormat(t, wrapBody(src))
		if !strings.Contains(got, want) {
			t.Fatalf("blank-slot render mismatch for %q:\n--got--\n%s\n--want line--\n%s", src, got, want)
		}
	}
}

// TestTupleBindTrailingCommaNormalized: a trailing comma in the pattern
// normalizes to the canonical comma-separated form (no trailing comma).
func TestTupleBindTrailingCommaNormalized(t *testing.T) {
	got := mustFormat(t, wrapBody("let (a: int, b: string,) = pair()"))
	want := "    let (a: int, b: string) = pair()\n"
	if !strings.Contains(got, want) {
		t.Fatalf("trailing-comma normalization mismatch:\n--got--\n%s\n--want line--\n%s", got, want)
	}
	if strings.Contains(got, "string,)") {
		t.Fatalf("trailing comma not stripped:\n%s", got)
	}
}

func TestTupleBindIdempotent(t *testing.T) {
	srcs := []string{
		wrapBody("let (a:int,b:string)=pair()"),
		wrapBody("final (_,out:string)=pair()"),
		wrapBody("let (a:int,b:string,)=pair()"),
		wrapBody("let (_:int,b:string[])=pair()"),
	}
	for _, src := range srcs {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Fatalf("not idempotent for %q:\n--once--\n%s\n--twice--\n%s", src, once, twice)
		}
	}
}

// TestSingleNameBindingUnchanged pins that a single-name let/final remains
// byte-identical to the existing rendering (no regression from the new case).
func TestSingleNameBindingUnchanged(t *testing.T) {
	cases := map[string]string{
		wrapBody("let a:int=f()"):   "    let a: int = f()\n",
		wrapBody("final a:int=f()"): "    final a: int = f()\n",
	}
	for src, want := range cases {
		got := mustFormat(t, src)
		if !strings.Contains(got, want) {
			t.Fatalf("single-name render mismatch for %q:\n--got--\n%s\n--want line--\n%s", src, got, want)
		}
	}
}
