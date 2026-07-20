package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSetStdin_TestIsolation proves the per-test-subshell property (Gap 1
// design, "Test isolation" section): set_stdin in one test() block does not
// leak into the next. The runner executes an entire *_test.wisp file as ONE
// shell process, each test body in its own `( ... )` subshell -- so `exec 0<`
// inside test A's subshell rebinds fd 0 only for that subshell, not the
// process's real fd 0 (which, under `go test`, is the null device: an
// immediate EOF).
//
// Test A sets a TWO-line buffer and reads only the first line, leaving "b"
// unread. If set_stdin leaked past the subshell boundary, test B's read_line()
// would see the leftover "b" line (Some("b")); properly isolated, test B reads
// the real process stdin (EOF) and gets None.
func TestSetStdin_TestIsolation(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	const src = `test ("first sets stdin and reads only one line") {
  set_stdin("a\nb\n")
  assert_eq(unwrap(read_line()), "a")
}

test ("second test is not affected by the first") {
  assert_none(read_line())
}
`
	dir := t.TempDir()
	p := filepath.Join(dir, "isolation_test.wisp")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0 (set_stdin must not leak across tests)\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
}
