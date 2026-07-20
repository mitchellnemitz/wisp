package token

import "testing"

func TestFinalKeyword(t *testing.T) {
	got, ok := Lookup("final")
	if !ok {
		t.Fatal("Lookup(\"final\"): expected keyword, got not-a-keyword")
	}
	if got != Final {
		t.Fatalf("Lookup(\"final\") = %v, want Final", got)
	}
}

func TestFinalKindString(t *testing.T) {
	if Final.String() != "final" {
		t.Fatalf("Final.String() = %q, want \"final\"", Final.String())
	}
}

func TestConstAlreadyKeyword(t *testing.T) {
	got, ok := Lookup("const")
	if !ok {
		t.Fatal("Lookup(\"const\"): expected keyword, got not-a-keyword")
	}
	if got != Const {
		t.Fatalf("Lookup(\"const\") = %v, want Const", got)
	}
}
