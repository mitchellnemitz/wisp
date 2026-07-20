package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIdempotentOverGoldenCorpus formats every compiling golden fixture twice
// and asserts format(format(x)) == format(x). The golden corpus covers every
// language construct across M1-M6, so this is a broad real-world idempotence
// guard beyond the hand-written corpus. Fixtures that do not parse (the
// compile-error fixtures) are skipped -- the formatter only handles valid
// source by contract.
func TestIdempotentOverGoldenCorpus(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/golden/*.wisp")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no golden fixtures found")
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".wisp")
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			once, err := Format(string(b), path)
			if err != nil {
				t.Skipf("does not parse (expected for compile-error fixtures): %v", err)
			}
			twice, err := Format(once, path)
			if err != nil {
				t.Fatalf("re-formatting failed: %v", err)
			}
			if once != twice {
				t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
			}
			// structural invariants on the real corpus too
			if strings.Contains(once, "\t") {
				t.Fatal("output contains a tab")
			}
			if strings.HasPrefix(once, "\n") {
				t.Fatal("leading blank line")
			}
			if once != "" && !strings.HasSuffix(once, "\n") {
				t.Fatal("missing trailing newline")
			}
		})
	}
}
