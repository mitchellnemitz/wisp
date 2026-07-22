package types

import (
	"strings"
	"testing"
)

func TestTaggedVariantAccessNotFolded(t *testing.T) {
	info := checkSrc(t, "enum E { A(int), B }\nfn f() -> int { let g: E = E.B\n return 0 }")
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "has no variant") {
			t.Fatalf("tagged variant E.B routed through the value-enum fold path: %q", d.Msg)
		}
	}
}
