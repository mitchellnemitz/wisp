package parser

import (
	"strings"
	"testing"
)

// RunResult and Process are built-in type-tag names (like Optional/Result);
// they must be rejected as type-parameter names at parse time just like the
// other built-in type words.
func TestTypeParam_RunResultProcess_Rejected(t *testing.T) {
	cases := []string{
		"fn foo[RunResult]() -> int { return 0 }",
		"fn foo[Process]() -> int { return 0 }",
		"struct Box[RunResult] { v: int }",
		"struct Box[Process] { v: int }",
	}
	for _, src := range cases {
		err := parseErr(t, src)
		if !strings.Contains(err.Error(), "collides with a built-in type name") {
			t.Errorf("parse(%q) error = %v, want collision error", src, err)
		}
	}
}
