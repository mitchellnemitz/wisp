package types

import (
	"strings"
	"testing"
)

func TestBareEnumExplicitValueSingleDiagnostic(t *testing.T) {
	info := checkSrc(t, "enum X { A = 0, B = 1 }\nfn main() -> int { return 0 }")
	if len(info.Errors) != 1 {
		t.Fatalf("FR-022 precedence: want exactly 1 diagnostic, got %d: %v", len(info.Errors), info.Errors)
	}
	if !strings.Contains(info.Errors[0].Msg, "explicit value") {
		t.Errorf("the single diagnostic should be the FR-022 add-a-backing error, got %q", info.Errors[0].Msg)
	}
}
