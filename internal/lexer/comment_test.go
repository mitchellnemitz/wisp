package lexer

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// TestLexByteStableWithComments verifies that LexWithComments produces exactly
// the same token stream (kinds, literals, positions) as Lex, so the parser and
// every existing caller see an unchanged stream regardless of comments.
func TestLexByteStableWithComments(t *testing.T) {
	srcs := []string{
		"fn main() -> int { return 0 }",
		"// lead\nfn main() -> int {\n  let x: int = 1 // trailing\n  return x\n}",
		"let // mid\nx",
	}
	for _, src := range srcs {
		base, err := Lex(src, "t.wisp")
		if err != nil {
			t.Fatalf("Lex(%q): %v", src, err)
		}
		toks, _, err := LexWithComments(src, "t.wisp")
		if err != nil {
			t.Fatalf("LexWithComments(%q): %v", src, err)
		}
		if len(toks) != len(base) {
			t.Fatalf("LexWithComments(%q): %d toks, Lex: %d", src, len(toks), len(base))
		}
		for i := range base {
			if toks[i] != base[i] {
				t.Fatalf("LexWithComments(%q) tok %d = %+v, want %+v", src, i, toks[i], base[i])
			}
		}
	}
}

func TestCommentsCaptureFullLine(t *testing.T) {
	src := "// a full-line comment\nfn main() -> int { return 0 }"
	_, comments, err := LexWithComments(src, "t.wisp")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1: %+v", len(comments), comments)
	}
	c := comments[0]
	if c.Text != "// a full-line comment" {
		t.Fatalf("text = %q", c.Text)
	}
	if c.Trailing {
		t.Fatalf("full-line comment marked Trailing")
	}
	if c.Pos.Line != 1 || c.Pos.Col != 1 {
		t.Fatalf("pos = %+v, want 1:1", c.Pos)
	}
}

func TestCommentsCaptureTrailing(t *testing.T) {
	src := "fn main() -> int {\n  return 0 // done\n}"
	_, comments, err := LexWithComments(src, "t.wisp")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
	c := comments[0]
	if c.Text != "// done" {
		t.Fatalf("text = %q", c.Text)
	}
	if !c.Trailing {
		t.Fatalf("trailing comment not marked Trailing")
	}
	if c.Pos.Line != 2 {
		t.Fatalf("pos line = %d, want 2", c.Pos.Line)
	}
}

func TestCommentsStackedKeepOrder(t *testing.T) {
	src := "// first\n// second\n// third\nfn main() -> int { return 0 }"
	_, comments, err := LexWithComments(src, "t.wisp")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"// first", "// second", "// third"}
	if len(comments) != len(want) {
		t.Fatalf("got %d comments, want %d", len(comments), len(want))
	}
	for i, w := range want {
		if comments[i].Text != w {
			t.Fatalf("comment %d = %q, want %q", i, comments[i].Text, w)
		}
		if comments[i].Trailing {
			t.Fatalf("comment %d marked trailing", i)
		}
		if comments[i].Pos.Line != i+1 {
			t.Fatalf("comment %d line = %d, want %d", i, comments[i].Pos.Line, i+1)
		}
	}
}

// TestCommentTextTrimmedTrailingSpace verifies the captured comment text drops
// trailing carriage-return / spaces so a CRLF source does not leave a stray \r
// in the comment body.
func TestCommentTextNoCarriageReturn(t *testing.T) {
	src := "// hi\r\nfn main() -> int { return 0 }"
	_, comments, err := LexWithComments(src, "t.wisp")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments", len(comments))
	}
	if comments[0].Text != "// hi" {
		t.Fatalf("text = %q, want %q", comments[0].Text, "// hi")
	}
}

// sanity: the Comment.Pos carries the file name through.
func TestCommentPosFile(t *testing.T) {
	_, comments, err := LexWithComments("// x\nfn main() -> int { return 0 }", "prog.wisp")
	if err != nil {
		t.Fatal(err)
	}
	if comments[0].Pos.File != "prog.wisp" {
		t.Fatalf("file = %q", comments[0].Pos.File)
	}
	_ = token.EOF
}
