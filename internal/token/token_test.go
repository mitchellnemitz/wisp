package token

import "testing"

func TestLookupKeyword(t *testing.T) {
	cases := []struct {
		lit  string
		want Kind
	}{
		{"let", Let},
		{"fn", Fn},
		{"return", Return},
		{"if", If},
		{"else", Else},
		{"while", While},
		{"for", For},
		{"switch", Switch},
		{"case", Case},
		{"default", Default},
		{"break", Break},
		{"continue", Continue},
		{"true", True},
		{"false", False},
		{"int", TypeInt},
		{"bool", TypeBool},
		{"string", TypeString},
		{"void", TypeVoid},
		// reserved-for-later keywords are still recognized as keywords
		{"struct", Struct},
		{"float", Float},
		{"try", Try},
		{"catch", Catch},
		{"finally", Finally},
		{"throw", Throw},
		{"error", Error},
		{"const", Const},
		{"import", Import},
		{"export", Export},
		{"include", Include},
		{"type", Type},
	}
	for _, c := range cases {
		got, ok := Lookup(c.lit)
		if !ok {
			t.Errorf("Lookup(%q): expected keyword, got not-a-keyword", c.lit)
			continue
		}
		if got != c.want {
			t.Errorf("Lookup(%q) = %v, want %v", c.lit, got, c.want)
		}
	}
}

func TestLookupNonKeyword(t *testing.T) {
	for _, lit := range []string{"foo", "x", "__ret", "main", "print", "Int", "LET"} {
		if k, ok := Lookup(lit); ok {
			t.Errorf("Lookup(%q): expected non-keyword, got keyword %v", lit, k)
		}
	}
}

func TestPositionString(t *testing.T) {
	p := Position{File: "foo.wisp", Line: 3, Col: 7}
	if got, want := p.String(), "foo.wisp:3:7"; got != want {
		t.Errorf("Position.String() = %q, want %q", got, want)
	}
	// no file
	p2 := Position{Line: 1, Col: 1}
	if got, want := p2.String(), "1:1"; got != want {
		t.Errorf("Position.String() = %q, want %q", got, want)
	}
}

func TestKindString(t *testing.T) {
	// Spot-check that String() is implemented and distinct for a few kinds.
	if Let.String() == Fn.String() {
		t.Errorf("expected distinct String() for Let and Fn")
	}
	if EOF.String() == "" {
		t.Errorf("expected non-empty String() for EOF")
	}
}
