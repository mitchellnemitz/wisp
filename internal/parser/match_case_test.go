package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// New `case <pattern> { body }` arm form parses and sets CasePos.
func TestMatchCaseArmParses(t *testing.T) {
	prog := parseOK(t, wrap(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { print(x) } case None { } }`))
	stmts := mainBody(t, prog)
	var m *ast.MatchStmt
	for _, s := range stmts {
		if ms, ok := s.(*ast.MatchStmt); ok {
			m = ms
		}
	}
	if m == nil {
		t.Fatalf("no match statement parsed")
	}
	if len(m.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(m.Arms))
	}
	for i, arm := range m.Arms {
		if arm.CasePos.Line == 0 {
			t.Errorf("arm %d: CasePos not set", i)
		}
	}
}

// The old `<pattern> => { body }` form is now a located parse error with a
// migration hint.
func TestMatchOldArrowFormRejected(t *testing.T) {
	err := parseErr(t, wrap(`let o: Optional[int] = Some(1)
match (o) { Some(x) => { } None => { } }`))
	if !strings.Contains(err.Error(), "case") {
		t.Errorf("expected migration hint mentioning `case`, got: %v", err)
	}
}

// A bare `case` with no pattern start is still rejected.
func TestMatchBareCaseNoPattern(t *testing.T) {
	parseErr(t, wrap(`let o: Optional[int] = Some(1)
match (o) { case => { } }`))
}
