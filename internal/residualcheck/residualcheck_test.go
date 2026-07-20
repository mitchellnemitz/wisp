package residualcheck

import (
	"fmt"
	"testing"
)

const repoRoot = "../.."

// TestNoResidualOldSpellings is the PR-A residual-usage gate required by the
// plan (docs/plans/2026-07-01-modules-only-universal-funcrefs.md, PR A
// scope): reintroducing the pre-rename namespace names (strings.*/arrays.*)
// or the pre-rename conversion spellings (string(x)/int(x)/float(x)/bool(x))
// as wisp-language syntax must fail `go test ./...`, so a regression cannot
// land silently before PR C's full completeness gate exists.
func TestNoResidualOldSpellings(t *testing.T) {
	violations, err := ScanRepo(repoRoot)
	if err != nil {
		t.Fatalf("ScanRepo: %v", err)
	}
	if len(violations) == 0 {
		return
	}
	msg := fmt.Sprintf("found %d residual old-spelling occurrence(s):\n", len(violations))
	for _, v := range violations {
		msg += fmt.Sprintf("  %s:%d [%s] %s\n", v.File, v.Line, v.Kind, v.Text)
	}
	msg += "\nIf this is a genuine new negative test asserting the old spelling is " +
		"rejected, add it to allowedSites in internal/residualcheck/residualcheck.go " +
		"instead of weakening the scan."
	t.Fatal(msg)
}
