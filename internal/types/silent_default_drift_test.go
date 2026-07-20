package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// NOTE: exprType's default (internal/types/expr.go) and genExpr's default
// (internal/codegen/expr.go) are the two genuinely UNTESTABLE-by-synthetic-value
// sites in this sweep: both switch over the sealed ast.Expr interface
// (exprNode() is unexported to internal/ast), and every one of its 14 concrete
// implementors already has an explicit case in both switches -- there is no
// real ast.Expr value external code can construct that would reach either
// default. Both defaults are exercised only by the full regression suite
// confirming they stay unreached; this is a documented, accepted testing
// limitation, not a coverage gap.

func TestCheckUnary_UnhandledOpFailsLoud(t *testing.T) {
	c := mkChecker()
	n := &ast.UnaryExpr{Op: token.Kind(9999), X: &ast.IntLit{Raw: "1"}}
	got := c.checkUnary(n)
	if got != Invalid {
		t.Fatalf("checkUnary with unhandled op = %v, want Invalid", got)
	}
	if len(c.info.Errors) == 0 {
		t.Fatalf("checkUnary with unhandled op recorded no diagnostic")
	}
	msg := c.info.Errors[len(c.info.Errors)-1].Msg
	if !strings.Contains(msg, "checker drift") {
		t.Fatalf("diagnostic %q does not name the checker-drift guard", msg)
	}
}

func TestCheckBinary_UnhandledOpFailsLoud(t *testing.T) {
	c := mkChecker()
	n := &ast.BinaryExpr{
		Op: token.Kind(9999),
		L:  &ast.IntLit{Raw: "1"},
		R:  &ast.IntLit{Raw: "2"},
	}
	got := c.checkBinary(n)
	if got != Invalid {
		t.Fatalf("checkBinary with unhandled op = %v, want Invalid", got)
	}
	if len(c.info.Errors) == 0 {
		t.Fatalf("checkBinary with unhandled op recorded no diagnostic")
	}
	msg := c.info.Errors[len(c.info.Errors)-1].Msg
	if !strings.Contains(msg, "checker drift") {
		t.Fatalf("diagnostic %q does not name the checker-drift guard", msg)
	}
}

func TestBinaryResultRecover_UnhandledOpPanics(t *testing.T) {
	const drift = token.Kind(9999)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("binaryResultRecover(%v) did not panic; the drift guard is missing", drift)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "binaryResultRecover") {
			t.Fatalf("panic message %q does not name binaryResultRecover", msg)
		}
	}()
	binaryResultRecover(drift)
}
