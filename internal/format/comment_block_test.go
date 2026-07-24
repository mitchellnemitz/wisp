package format

import "testing"

// TestCommentEndOfThenBlock (H8): a full-line comment that is the last thing in
// a then-block (after the block's last statement, before `}`) must stay inside
// the then-block at the then-block's indent, not leak into the else branch.
func TestCommentEndOfThenBlock(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    if (true) {\n" +
		"        let x: int = 1\n" +
		"        // end of then\n" +
		"    } else {\n" +
		"        let y: int = 2\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := "fn main() -> int {\n" +
		"    if (true) {\n" +
		"        let x: int = 1\n" +
		"        // end of then\n" +
		"    } else {\n" +
		"        let y: int = 2\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("H8 end-of-then comment:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("H8 not idempotent:\n--once--\n%s\n--twice--\n%s", got, mustFormat(t, got))
	}
}

// TestCommentEndOfBlockBeforeStatement (H8): a comment that is the last thing in
// a nested block must stay in that block, not be relocated onto the following
// outer statement.
func TestCommentEndOfBlockBeforeStatement(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    while (true) {\n" +
		"        let x: int = 1\n" +
		"        // tail of loop\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := "fn main() -> int {\n" +
		"    while (true) {\n" +
		"        let x: int = 1\n" +
		"        // tail of loop\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("H8 end-of-loop comment:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("H8 loop not idempotent")
	}
}

// TestCommentSoleFunctionBody (H9): a comment that is the only content of a
// function body must stay inside that body, not escape onto the next top-level
// declaration.
func TestCommentSoleFunctionBody(t *testing.T) {
	src := "fn noop() -> void {\n" +
		"    // nothing to do yet\n" +
		"}\n" +
		"fn main() -> int {\n" +
		"    return 0\n" +
		"}\n"
	want := "fn noop() -> void {\n" +
		"    // nothing to do yet\n" +
		"}\n" +
		"\n" +
		"fn main() -> int {\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("H9 sole-body comment:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("H9 not idempotent:\n--once--\n%s\n--twice--\n%s", got, mustFormat(t, got))
	}
}

// TestCommentBeforeElseStaysWithThen (M8): a full-line comment sitting just
// before `} else` belongs to the preceding then-branch, not pulled into the
// else body.
func TestCommentBeforeElseStaysWithThen(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    if (true) {\n" +
		"        let a: int = 1\n" +
		"        // trailing then note\n" +
		"    } else if (false) {\n" +
		"        let b: int = 2\n" +
		"        // trailing elseif note\n" +
		"    } else {\n" +
		"        let c: int = 3\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("M8 comment before else:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("M8 not idempotent")
	}
}

// TestCommentBeforeCatchStaysWithTry (M8): a comment just before `} catch`
// stays in the try body; one before `} finally` stays in the catch body.
func TestCommentBeforeCatchStaysWithTry(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    try {\n" +
		"        print(\"body\")\n" +
		"        // end of try\n" +
		"    } catch (e) {\n" +
		"        print(e.message)\n" +
		"        // end of catch\n" +
		"    } finally {\n" +
		"        print(\"cleanup\")\n" +
		"        // end of finally\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("M8 try/catch/finally tail comments:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("M8 try not idempotent")
	}
}

// TestTrailingCommentOnElseBrace (M8): a comment ON the `} else if`/`} else`
// continuation-brace line stays on that line, not swallowed into the branch body.
func TestTrailingCommentOnElseBrace(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    if (true) {\n" +
		"        let a: int = 1\n" +
		"    } else if (false) { // on elseif\n" +
		"        let b: int = 2\n" +
		"    } else { // on else\n" +
		"        let c: int = 3\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("M8 trailing comment on else brace:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("M8 else-brace not idempotent:\n--once--\n%s\n--twice--\n%s", got, mustFormat(t, got))
	}
}

// TestTrailingCommentOnCatchFinallyBrace (M8): a comment ON the `} catch`/
// `} finally` continuation-brace line stays on that line.
func TestTrailingCommentOnCatchFinallyBrace(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    try {\n" +
		"        print(\"body\")\n" +
		"    } catch (e) { // on catch\n" +
		"        print(e.message)\n" +
		"    } finally { // on finally\n" +
		"        print(\"cleanup\")\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("M8 trailing comment on catch/finally brace:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("M8 catch/finally-brace not idempotent:\n--once--\n%s\n--twice--\n%s", got, mustFormat(t, got))
	}
}

// TestCommentEndOfMatchArm (M8/H8): a comment at the tail of a match arm body
// stays inside that arm.
func TestCommentEndOfMatchArm(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let r: Result[int] = Ok(1)\n" +
		"    match (r) {\n" +
		"        case Ok(v) {\n" +
		"            print(\"${v}\")\n" +
		"            // end of ok arm\n" +
		"        }\n" +
		"        case Err(e) {\n" +
		"            print(e.message)\n" +
		"        }\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("M8 match-arm tail comment:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("M8 match not idempotent")
	}
}

// TestDocCommentDoesNotLeakIntoPriorFuncBody (B2): a `///` doc comment that
// precedes the SECOND of two top-level functions must stay attached to that
// second function, not get swept into the first function's body as dead code
// after its `return`. It also pins the documented blank-line behavior for the
// same source (www/src/content/docs/guide/cli.md: blank lines within a block are removed; one
// blank line separates top-level declarations) so the two concerns aren't
// conflated.
func TestDocCommentDoesNotLeakIntoPriorFuncBody(t *testing.T) {
	src := "/// greet returns a greeting.\n" +
		"export fn greet(name: string) -> string {\n" +
		"  let msg: string = \"hi\"\n" +
		"\n" +
		"  return msg\n" +
		"}\n" +
		"\n" +
		"/// shout uppercases.\n" +
		"export fn shout(s: string) -> string {\n" +
		"  return s\n" +
		"}\n"
	want := "/// greet returns a greeting.\n" +
		"export fn greet(name: string) -> string {\n" +
		"    let msg: string = \"hi\"\n" +
		"    return msg\n" +
		"}\n" +
		"\n" +
		"/// shout uppercases.\n" +
		"export fn shout(s: string) -> string {\n" +
		"    return s\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("B2 doc comment leaked into prior body:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("B2 not idempotent:\n--once--\n%s\n--twice--\n%s", got, mustFormat(t, got))
	}
}

// TestCommentEndOfSwitchCase (H8): a comment at the tail of a switch case body
// stays inside that case; one at the tail of default stays in default.
func TestCommentEndOfSwitchCase(t *testing.T) {
	src := "fn main() -> int {\n" +
		"    let x: int = 1\n" +
		"    switch (x) {\n" +
		"        case 1, 2 {\n" +
		"            print(\"low\")\n" +
		"            // end of case\n" +
		"        }\n" +
		"        default {\n" +
		"            print(\"hi\")\n" +
		"            // end of default\n" +
		"        }\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	want := src
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("H8 switch-case tail comment:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if got != mustFormat(t, got) {
		t.Fatalf("H8 switch not idempotent")
	}
}
