package codegen

import (
	"strings"
	"testing"
)

// TestMatchOptionalBehavioral: Some(x) arm prints value; None arm prints "none".
func TestMatchOptionalBehavioral(t *testing.T) {
	src := "fn main() -> int {\n" +
		"let s: Optional[int] = Some(42)\n" +
		"match (s) {\n" +
		"  case Some(x) { print(to_string(x)) }\n" +
		"  case None { print(\"none\") }\n" +
		"}\n" +
		"let n: Optional[int] = None\n" +
		"match (n) {\n" +
		"  case Some(x) { print(to_string(x)) }\n" +
		"  case None { print(\"none\") }\n" +
		"}\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "42\nnone\n" {
		t.Errorf("stdout = %q, want %q", stdout, "42\nnone\n")
	}
}

// TestMatchResultBehavioral: Ok(v) arm prints value; Err(e) arm prints message.
func TestMatchResultBehavioral(t *testing.T) {
	src := "fn main() -> int {\n" +
		"let ok: Result[int] = Ok(7)\n" +
		"match (ok) {\n" +
		"  case Ok(v) { print(to_string(v)) }\n" +
		"  case Err(e) { print(e.message) }\n" +
		"}\n" +
		"let bad: Result[int] = Err(error(\"boom\"))\n" +
		"match (bad) {\n" +
		"  case Ok(v) { print(to_string(v)) }\n" +
		"  case Err(e) { print(e.message) }\n" +
		"}\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "7\nboom\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\nboom\n")
	}
}

// TestMatchWildcardLastBehavioral: wildcard arm catches remaining variants.
func TestMatchWildcardLastBehavioral(t *testing.T) {
	src := "fn main() -> int {\n" +
		"let s: Optional[int] = Some(1)\n" +
		"match (s) {\n" +
		"  case Some(x) { print(\"some\") }\n" +
		"  case _ { print(\"other\") }\n" +
		"}\n" +
		"let n: Optional[int] = None\n" +
		"match (n) {\n" +
		"  case Some(x) { print(\"some\") }\n" +
		"  case _ { print(\"other\") }\n" +
		"}\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "some\nother\n" {
		t.Errorf("stdout = %q, want %q", stdout, "some\nother\n")
	}
}

// TestMatchSingleWildcardBehavioral: a lone wildcard arm covers all variants.
func TestMatchSingleWildcardBehavioral(t *testing.T) {
	src := wrapMainCG(
		"let o: Optional[int] = Some(3)\n" +
			"match (o) {\n" +
			"  case _ { print(\"any\") }\n" +
			"}")
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "any\n" {
		t.Errorf("stdout = %q, want %q", stdout, "any\n")
	}
}

// TestMatchDiscardBehavioral: Some(_) discards the payload without binding.
func TestMatchDiscardBehavioral(t *testing.T) {
	src := "fn main() -> int {\n" +
		"let s: Optional[int] = Some(99)\n" +
		"match (s) {\n" +
		"  case Some(_) { print(\"present\") }\n" +
		"  case None { print(\"absent\") }\n" +
		"}\n" +
		"let n: Optional[int] = None\n" +
		"match (n) {\n" +
		"  case Some(_) { print(\"present\") }\n" +
		"  case None { print(\"absent\") }\n" +
		"}\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "present\nabsent\n" {
		t.Errorf("stdout = %q, want %q", stdout, "present\nabsent\n")
	}
}

// TestMatchCallScrutineeBehavioral: a call scrutinee is evaluated exactly once.
func TestMatchCallScrutineeBehavioral(t *testing.T) {
	src := "fn make() -> Optional[int] {\nprint(\"called\")\nreturn Some(5)\n}\n" +
		wrapMainCG(
			"match (make()) {\n"+
				"  case Some(x) { print(to_string(x)) }\n"+
				"  case None { print(\"none\") }\n"+
				"}")
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "called\n5\n" {
		t.Errorf("stdout = %q, want exactly one 'called' then '5', got %q", stdout, "called\n5\n")
	}
}

// TestMatchShape: generated shell must use the _tag field for Optional match.
func TestMatchOptionalShape(t *testing.T) {
	src := wrapMainCG(
		"let o: Optional[int] = Some(1)\n" +
			"match (o) {\n" +
			"  case Some(x) { print(to_string(x)) }\n" +
			"  case None { }\n" +
			"}")
	out := string(compile(t, src))
	if !strings.Contains(out, "}_tag") {
		t.Errorf("match Optional: expected _tag check, got:\n%s", out)
	}
	if !strings.Contains(out, "if ") {
		t.Errorf("match Optional: expected conditional (if), got:\n%s", out)
	}
}

// TestMatchResultShape: generated shell must use _tag for Result match.
func TestMatchResultShape(t *testing.T) {
	src := wrapMainCG(
		"let r: Result[int] = Ok(1)\n" +
			"match (r) {\n" +
			"  case Ok(v) { print(to_string(v)) }\n" +
			"  case Err(e) { print(e.message) }\n" +
			"}")
	out := string(compile(t, src))
	if !strings.Contains(out, "}_tag") {
		t.Errorf("match Result: expected _tag field, got:\n%s", out)
	}
}

// TestMatchGenericMonoBehavioral: match inside a numeric-generic function exercises
// the mono.go and reachable.go MatchStmt walkers.
func TestMatchGenericMonoBehavioral(t *testing.T) {
	src := "fn has_val[T: numeric](o: Optional[T]) -> bool {\n" +
		"match (o) {\n" +
		"  case Some(x) { return true }\n" +
		"  case None { return false }\n" +
		"}\n}\n" +
		"fn main() -> int {\n" +
		"let a: Optional[int] = Some(3)\n" +
		"let b: Optional[int] = None\n" +
		"print(to_string(has_val(a)))\n" +
		"print(to_string(has_val(b)))\n" +
		"return 0\n}"
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "true\nfalse\n" {
		t.Errorf("stdout = %q, want %q", stdout, "true\nfalse\n")
	}
}
