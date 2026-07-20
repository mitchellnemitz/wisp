package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func TestParseThrow(t *testing.T) {
	prog := parseOK(t, wrap(`throw error("boom")`))
	body := mainBody(t, prog)
	ts, ok := body[0].(*ast.ThrowStmt)
	if !ok {
		t.Fatalf("stmt 0 is %T, want *ast.ThrowStmt", body[0])
	}
	call, ok := ts.X.(*ast.CallExpr)
	if !ok {
		t.Fatalf("throw operand is %T, want *ast.CallExpr", ts.X)
	}
	if call.CalleeName != "error" {
		t.Fatalf("callee = %q, want error", call.CalleeName)
	}
	if ts.KwPos.Line != 2 {
		t.Fatalf("throw KwPos line = %d, want 2", ts.KwPos.Line)
	}
}

func TestParseTryCatch(t *testing.T) {
	prog := parseOK(t, wrap("try {\n    print(\"body\")\n  } catch (e) {\n    print(\"handler\")\n  }"))
	body := mainBody(t, prog)
	ts, ok := body[0].(*ast.TryStmt)
	if !ok {
		t.Fatalf("stmt 0 is %T, want *ast.TryStmt", body[0])
	}
	if ts.CatchVar != "e" {
		t.Fatalf("catch var = %q, want e", ts.CatchVar)
	}
	if len(ts.Body) != 1 || len(ts.Catch) != 1 {
		t.Fatalf("body=%d catch=%d, want 1/1", len(ts.Body), len(ts.Catch))
	}
	if ts.HasFinally {
		t.Fatalf("HasFinally = true, want false (no finally clause)")
	}
}

func TestParseTryCatchFinally(t *testing.T) {
	prog := parseOK(t, wrap("try {\n    print(\"b\")\n  } catch (e) {\n    print(\"c\")\n  } finally {\n    print(\"f\")\n  }"))
	body := mainBody(t, prog)
	ts := body[0].(*ast.TryStmt)
	if !ts.HasFinally {
		t.Fatalf("HasFinally = false, want true")
	}
	if len(ts.Finally) != 1 {
		t.Fatalf("finally body = %d, want 1", len(ts.Finally))
	}
}

func TestParseTryEmptyFinally(t *testing.T) {
	prog := parseOK(t, wrap("try {\n    print(\"b\")\n  } catch (e) {\n    print(\"c\")\n  } finally {\n  }"))
	ts := mainBody(t, prog)[0].(*ast.TryStmt)
	if !ts.HasFinally {
		t.Fatalf("HasFinally = false, want true for an empty finally clause")
	}
	if len(ts.Finally) != 0 {
		t.Fatalf("finally body = %d, want 0", len(ts.Finally))
	}
}

func TestParseTryWithoutCatchErrors(t *testing.T) {
	err := parseErr(t, wrap("try {\n    print(\"b\")\n  }"))
	errAt(t, err, "catch")
}

func TestParseCatchRequiresParenVar(t *testing.T) {
	err := parseErr(t, wrap("try {\n  } catch {\n  }"))
	errAt(t, err, "(")
}

// TestIfLetIsRemovedParsesAsError pins that `if let` is no longer a recognized
// construct: `let` after `if` is now a parse error because ordinary `if`
// requires `(` immediately after the keyword.
func TestIfLetIsRemovedParsesAsError(t *testing.T) {
	err := parseErr(t, wrap("if let Some(x) = v { }"))
	// Pin the REASON: `if` requires `(` immediately after the keyword, so `let`
	// there is rejected by the condition parser. Asserting the specific message
	// guards against a future regression where `if let` fails for an unrelated
	// reason (which a bare "parsing failed" check would not catch).
	errAt(t, err, "expected (")
}
