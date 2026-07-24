package runtime

import (
	"strings"
	"testing"
)

// The fractional-output float helpers are the one awk family in this file
// that formats a decimal number for output (`printf "%.17g"` or
// `printf "%." d "f"`), so they are the one family whose output is sensitive
// to the caller's LC_NUMERIC. Every other awk-using helper in this file
// (string/regex/json/scmp) is already pinned to LC_ALL=C; this test locks
// the same guarantee in for the float family, so a comma-decimal locale
// (de_DE, fr_FR, ...) cannot make float output silently wrong or float
// arithmetic spuriously abort.
func TestFloatHelpers_AreLocaleIndependentViaAwk(t *testing.T) {
	for _, id := range []string{
		FAdd, FSub, FMul, FDiv,
		Sqrt, FormatFloat, Pow, Ln, Exp,
		FStr, FFloatI, FFloatS, FAbs,
	} {
		h, ok := registry[id]
		if !ok {
			t.Fatalf("float helper %q missing from registry", id)
		}
		if !strings.Contains(h.src, "LC_ALL=C awk") {
			t.Errorf("%s body missing `LC_ALL=C awk` (locale-independence guarantee for float output)", id)
		}
	}
}
