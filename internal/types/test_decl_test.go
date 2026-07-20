package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// checkFile parses src under filename and runs the checker, failing on a parse
// error. It lets a test choose a `*_test.wisp` filename so the `test (...)`
// construct is accepted by the parser.
func checkFile(t *testing.T, src, filename string) *Info {
	t.Helper()
	prog, err := parser.Parse(src, filename)
	if err != nil {
		t.Fatalf("parse error: %v\nsrc:\n%s", err, src)
	}
	return Check(prog)
}

// TestNoMainTestFileTypeChecks: a `*_test.wisp` with no `fn main` is valid (R2,
// AC15). The no-main requirement is suppressed for test files.
func TestNoMainTestFileTypeChecks(t *testing.T) {
	info := checkFile(t, `test ("adds") {
  let n: int = 1
}`, "calc_test.wisp")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors for a no-main test file, got:\n%s", diagList(info.Errors))
	}
}

// TestTestBodyTypeChecked: the body is checked like a `-> void` scope; a type
// error inside it is reported.
func TestTestBodyTypeChecked(t *testing.T) {
	info := checkFile(t, `test ("bad") {
  let n: int = "x"
}`, "calc_test.wisp")
	var found bool
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "int") && strings.Contains(d.Msg, "string") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an int/string type-mismatch error in the test body, got:\n%s", diagList(info.Errors))
	}
}

// TestDuplicateTestNameIsError: two tests with the same name in one file is a
// compile error (R3, AC13).
func TestDuplicateTestNameIsError(t *testing.T) {
	info := checkFile(t, `test ("same") {
}
test ("same") {
}`, "calc_test.wisp")
	var found bool
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "same") && strings.Contains(d.Msg, "more than once") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a duplicate-test-name error, got:\n%s", diagList(info.Errors))
	}
}

// TestDistinctTestNamesOK: distinct names in one file are fine.
func TestDistinctTestNamesOK(t *testing.T) {
	info := checkFile(t, `test ("a") {
}
test ("b") {
}`, "calc_test.wisp")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors for distinct test names, got:\n%s", diagList(info.Errors))
	}
}

// TestTestFileMayAlsoHaveMain: a test file with a main is still valid (main is
// allowed, just not required).
func TestTestFileWithFuncTypeChecks(t *testing.T) {
	info := checkFile(t, `fn helper() -> int {
  return 1
}
test ("uses helper") {
  let n: int = helper()
}`, "calc_test.wisp")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s", diagList(info.Errors))
	}
}
