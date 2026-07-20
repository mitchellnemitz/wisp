package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/lexer"
)

// newTestParser builds a parser the same way Parse does (lexed tokens, not
// a bare &parser{}) -- enterDepth's error path calls p.errHere -> p.cur()
// -> p.toks[p.pos], which panics on a nil toks slice.
func newTestParser(t *testing.T) *parser {
	t.Helper()
	toks, err := lexer.Lex("", "test.wisp")
	if err != nil {
		t.Fatalf("lexer.Lex(\"\", ...) unexpected error: %v", err)
	}
	return &parser{toks: toks, file: "test.wisp"}
}

func TestDepthGuard_ExactCap_Boundary(t *testing.T) {
	p := newTestParser(t)
	for i := 0; i < maxParseDepth; i++ {
		_, err := p.enterDepth()
		if err != nil {
			t.Fatalf("enterDepth() call %d: unexpected error: %v", i+1, err)
		}
	}
	_, err := p.enterDepth()
	if err == nil {
		t.Fatalf("enterDepth() call %d: expected error, got none", maxParseDepth+1)
	}
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("enterDepth() call %d: expected \"nesting too deep\" error, got: %v", maxParseDepth+1, err)
	}
}

func TestDepthGuard_DeepParens_Rejected(t *testing.T) {
	src := wrap("return " + strings.Repeat("(", 6000) + "1" + strings.Repeat(")", 6000))
	err := parseErr(t, src)
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("expected \"nesting too deep\" error, got: %v", err)
	}
}

func TestDepthGuard_DeepUnary_Rejected(t *testing.T) {
	src := wrap("return " + strings.Repeat("!", 6000) + "true")
	err := parseErr(t, src)
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("expected \"nesting too deep\" error, got: %v", err)
	}
}

func TestDepthGuard_ModerateNesting_StillOK(t *testing.T) {
	src := wrap("return " + strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200))
	parseOK(t, src)
}

func TestDepthGuard_DeepOptionalType_Rejected(t *testing.T) {
	inner := "int"
	for i := 0; i < 6000; i++ {
		inner = "Optional[" + inner + "]"
	}
	src := "fn f(x: " + inner + ") -> int {\n  return 0\n}"
	err := parseErr(t, src)
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("expected \"nesting too deep\" error, got: %v", err)
	}
}

func TestDepthGuard_DeepBlocks_Rejected(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 6000; i++ {
		b.WriteString("if (true) {\n")
	}
	b.WriteString("return 0\n")
	for i := 0; i < 6000; i++ {
		b.WriteString("}\n")
	}
	src := wrap(b.String())
	err := parseErr(t, src)
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("expected \"nesting too deep\" error, got: %v", err)
	}
}

// TestDepthGuard_MixedChains_Rejected proves the guard uses one shared
// counter, not independent per-construct counters: ~2000 nested if-blocks
// alone (persistent depth ~2001, dominated by parseBlock) stay under
// maxParseDepth, and ~2000 nested parens alone (persistent depth ~4000,
// since each paren level's parseExpr+parseUnary remain simultaneously on
// the call stack) also stay under maxParseDepth -- but nesting the parens
// inside the innermost if-block's body combines to ~6000, exceeding the
// 5000 cap.
func TestDepthGuard_MixedChains_Rejected(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		b.WriteString("if (true) {\n")
	}
	b.WriteString("return " + strings.Repeat("(", 2000) + "1" + strings.Repeat(")", 2000) + "\n")
	for i := 0; i < 2000; i++ {
		b.WriteString("}\n")
	}
	src := wrap(b.String())
	err := parseErr(t, src)
	if !strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("expected \"nesting too deep\" error, got: %v", err)
	}
}
