package parser

import "testing"

func TestParseEnumBackingAndPayload(t *testing.T) {
	prog, err := Parse(`enum Code: int { Ok = 0, Fail = 1 }
enum Expr { IntLit(int), Ident(string), Unit }`, "t.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(prog.Enums) != 2 {
		t.Fatalf("want 2 enums, got %d", len(prog.Enums))
	}
	code := prog.Enums[0]
	if code.Backing != "int" {
		t.Errorf("Code.Backing = %q, want int", code.Backing)
	}
	if code.Variants[0].Payload != "" {
		t.Errorf("Code.Ok.Payload = %q, want empty", code.Variants[0].Payload)
	}
	expr := prog.Enums[1]
	if expr.Backing != "" {
		t.Errorf("Expr.Backing = %q, want empty (bare)", expr.Backing)
	}
	if expr.Variants[0].Payload != "int" {
		t.Errorf("Expr.IntLit.Payload = %q, want int", expr.Variants[0].Payload)
	}
	if expr.Variants[2].Payload != "" {
		t.Errorf("Expr.Unit.Payload = %q, want empty", expr.Variants[2].Payload)
	}
}

func TestParseEnumGenericParams(t *testing.T) {
	// wisp spells type parameters with square brackets (`struct Box[T]`,
	// `fn f[T]`); enum generics follow suit. The declaration must PARSE so the
	// checker can reject it with a located FR-019 error, not a parse error.
	prog, err := Parse(`enum Box[T] { Full(T), Empty }`, "t.wisp")
	if err != nil {
		t.Fatalf("parse error (generic enum must PARSE so the checker can reject it): %v", err)
	}
	if len(prog.Enums[0].TypeParams) != 1 || prog.Enums[0].TypeParams[0] != "T" {
		t.Errorf("TypeParams = %v, want [T]", prog.Enums[0].TypeParams)
	}
}

func TestParseEnumMissingCommaFails(t *testing.T) {
	// SC-046: variant list without a separating comma is a parse error.
	if _, err := Parse(`enum Code: int { Ok = 0 Fail = 1 }`, "t.wisp"); err == nil {
		t.Fatal("want parse error for missing comma between variants")
	}
}
