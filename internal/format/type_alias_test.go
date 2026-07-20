package format

import (
	"strings"
	"testing"
)

func TestFormatTypeAlias(t *testing.T) {
	cases := map[string]string{
		"type Miles=int\n":          "type Miles = int",
		"type Names=string[]\n":     "type Names = string[]",
		"type B=fn(int,int)->int\n": "type B = fn(int, int) -> int",
		"type M={string:string}\n":  "type M = {string: string}",
		"type AA=Miles[]\n":         "type AA = Miles[]",
		"type P=(int,int)\n":        "type P = (int, int)",
	}
	for src, wantLine := range cases {
		got := mustFormat(t, src+"fn main() -> int { return 0 }\n")
		if !strings.Contains(got, wantLine) {
			t.Errorf("format(%q): want a line %q, got:\n%s", src, wantLine, got)
		}
		if mustFormat(t, got) != got {
			t.Errorf("not idempotent for %q:\n%s", src, got)
		}
	}
}

// TestFormatTypeAliasSourceOrder: an alias declared between two functions emits
// between them (position-ordered top-level emit).
func TestFormatTypeAliasSourceOrder(t *testing.T) {
	src := "fn a() -> int { return 1 }\ntype Mid = int\nfn b() -> int { return 2 }\nfn main() -> int { return 0 }\n"
	got := mustFormat(t, src)
	ia := strings.Index(got, "fn a(")
	im := strings.Index(got, "type Mid = int")
	ib := strings.Index(got, "fn b(")
	if !(ia < im && im < ib) {
		t.Errorf("expected order a < Mid < b, got:\n%s", got)
	}
}
