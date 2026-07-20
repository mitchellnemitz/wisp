package golden

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// stripComments removes `//` line-comment text while preserving every code
// token's exact line and column: a full-line comment becomes an empty line (its
// newline is kept), and a trailing comment is dropped from the end of its line
// (code before it keeps its position). This yields an "uncommented twin" whose
// code tokens occupy identical source positions, so any byte difference in the
// compiled script must come from the comment bytes leaking into codegen -- which
// B1 forbids. (A naive comment strip that also re-flowed lines would shift
// positions and make the located-abort line numbers legitimately differ.)
func stripComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = strings.TrimRight(line[:idx], " \t")
		}
	}
	return strings.Join(lines, "\n")
}

// TestCommentTwinByteIdentical verifies B1's core invariant: retaining `//`
// comments on the lexer side channel does NOT change compile/codegen output.
func TestCommentTwinByteIdentical(t *testing.T) {
	commented := `// top-of-file comment
// stacked second line
fn add(a: int, b: int) -> int { // trailing on signature
  // body comment
  return a + b // trailing on return
}

fn main() -> int {
  let x: int = add(2, 3) // call
  // a full-line comment before print
  print("sum=${x}")
  let xs: int[] = [1, 2, 3] // an array
  print("first=${xs[0]}") // index, which can abort located
  return 0
}
`
	uncommented := stripComments(commented)

	cScript, cMap, cDiags := driver.Compile("twin.wisp", commented)
	uScript, uMap, uDiags := driver.Compile("twin.wisp", uncommented)

	for _, d := range cDiags {
		if d.Severity == driver.Error {
			t.Fatalf("commented program failed to compile: %s", d)
		}
	}
	for _, d := range uDiags {
		if d.Severity == driver.Error {
			t.Fatalf("uncommented program failed to compile: %s", d)
		}
	}
	if !bytes.Equal(cScript, uScript) {
		t.Fatalf("script bytes differ between commented and uncommented twin:\n--commented--\n%s\n--uncommented--\n%s", cScript, uScript)
	}
	if len(cMap) != len(uMap) {
		t.Fatalf("source map length differs: %d vs %d", len(cMap), len(uMap))
	}
	for i := range cMap {
		if (cMap[i] == nil) != (uMap[i] == nil) {
			t.Fatalf("source map entry %d nil-ness differs", i)
		}
		if cMap[i] != nil && *cMap[i] != *uMap[i] {
			t.Fatalf("source map entry %d differs: %+v vs %+v", i, *cMap[i], *uMap[i])
		}
	}
}
