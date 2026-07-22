package format

import (
	"strings"
	"testing"
)

func TestFormatEnumBackingAndPayload(t *testing.T) {
	cases := []string{
		"enum Code: int { Ok = 0, Fail = 1 }\n",
		"enum Expr {\n  IntLit(int),\n  Unit,\n}\n",
	}
	for _, src := range cases {
		out, err := Format(src, "t.wisp") // internal/format/format.go:26 -- the entry point cmd/wisp fmt uses
		if err != nil {
			t.Fatalf("format error for %q: %v", src, err)
		}
		again, err := Format(out, "t.wisp")
		if err != nil {
			t.Fatalf("re-format error: %v", err)
		}
		if out != again {
			t.Errorf("format not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
		}
		if !strings.Contains(out, "Code: int") && !strings.Contains(out, "IntLit(int)") {
			t.Errorf("formatted output dropped backing/payload: %q", out)
		}
	}
}
