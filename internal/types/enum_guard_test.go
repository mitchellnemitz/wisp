package types

import (
	"strings"
	"testing"
)

// TestTaggedVariantAccessNotFolded guards against the tagged-variant defect:
// collectTaggedEnum seeds ei.Consts[i] = nil for a tagged variant, so
// constValue still reports a hit (nil, true) for E.B. Without the
// EnumTagged early-return guard in checkEnumVariantAccess, the value-enum
// fold path silently accepts that nil hit and records
// info.FoldedValues[node] = nil for the E.B access - no diagnostic is
// produced, but a tagged-union variant access is incorrectly treated as a
// folded constant. The guard must prevent any fold from being recorded for
// this access. This source has no value enum, so E.B is the only possible
// fold site: info.FoldedValues must end up empty.
func TestTaggedVariantAccessNotFolded(t *testing.T) {
	info := checkSrc(t, "enum E { A(int), B }\nfn f() -> int { let g: E = E.B\n return 0 }")
	if n := len(info.FoldedValues); n != 0 {
		t.Fatalf("info.FoldedValues has %d entries, want 0 (tagged variant E.B must not be const-folded)", n)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "has no variant") {
			t.Fatalf("tagged variant E.B routed through the value-enum fold path: %q", d.Msg)
		}
	}
}
