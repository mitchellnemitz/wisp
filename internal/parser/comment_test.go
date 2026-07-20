package parser

import "testing"

// TestParseWithCommentsSameProgram verifies ParseWithComments yields the same
// number of top-level decls as Parse (the AST is unaffected by comments) and
// surfaces the comments in source order.
func TestParseWithCommentsSameProgram(t *testing.T) {
	src := "// header\n" +
		"fn main() -> int {\n" +
		"  let x: int = 1 // init\n" +
		"  return x\n" +
		"}\n"

	base, err := Parse(src, "t.wisp")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	prog, comments, err := ParseWithComments(src, "t.wisp")
	if err != nil {
		t.Fatalf("ParseWithComments: %v", err)
	}
	if len(prog.Funcs) != len(base.Funcs) || len(prog.Structs) != len(base.Structs) {
		t.Fatalf("decl counts differ: %d/%d vs %d/%d",
			len(prog.Funcs), len(prog.Structs), len(base.Funcs), len(base.Structs))
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments, want 2", len(comments))
	}
	if comments[0].Text != "// header" || comments[0].Trailing {
		t.Fatalf("comment 0 = %+v", comments[0])
	}
	if comments[1].Text != "// init" || !comments[1].Trailing {
		t.Fatalf("comment 1 = %+v", comments[1])
	}
}

// TestParseWithCommentsErrorNilComments verifies a parse error yields a nil
// comment slice and no program.
func TestParseWithCommentsErrorNilComments(t *testing.T) {
	prog, comments, err := ParseWithComments("fn main( -> int {}", "t.wisp")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if prog != nil || comments != nil {
		t.Fatalf("on error want nil prog/comments, got %v / %v", prog, comments)
	}
}
