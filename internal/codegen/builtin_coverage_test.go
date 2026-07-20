package codegen

// This file is the fifth-surface gate. Adding a builtin touches five surfaces
// (AGENTS.md:81-82); four already have machine gates -- editor grammars
// (internal/editors/editors_test.go), the checker's tables
// (internal/types/accessors_test.go), funcref-wrapper bidirectional pinning
// (internal/codegen/builtinref_consistency_test.go), and codegen->prelude
// helper linkage (Go-compile-time). The fifth -- the checker->codegen direct
// call dispatch arm in genBuiltinCall (internal/codegen/expr.go) -- had no
// pre-merge gate: its only guard, the T1.1 drift panic
// (TestGenBuiltinCall_UnhandledBuiltinPanics in builtin_drift_test.go), fires
// only when some test or user actually compiles a call to the builtin, at
// runtime, not at `go test ./...` time.
//
// This test closes that gap cheaply: it asserts every types.BuiltinNames()
// entry appears as a whole-word identifier somewhere in the compiled-test
// corpus (every testdata/golden/**/*.wisp file, or a string literal in any
// internal/codegen/*_test.go file other than this one). It proves "the name
// is present in a compiled source," not "the call type-checks and runs" --
// synthesizing a real call per builtin founders on composite-arg specials
// (see the design doc's Non-goals).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/types"
)

// allowedUncovered documents builtin names that legitimately cannot be given
// a compiled-test occurrence, keyed by name -> a written justification
// explaining why. Adding an entry requires a non-empty justification --
// machine-enforced below (TestBuiltinDispatch_AllNamesCovered iterates this
// map and fails on any empty/whitespace-only value), not just doc-comment
// convention. The map's value type is string, not bool, specifically so an
// empty value is visibly wrong in a diff. Starts empty: every builtin the
// gate's initial probe found uncovered (assert_err) is closed with a real
// test, not allowlisted here -- this mechanism exists for a future
// legitimately uncoverable builtin, not because this task needs one.
var allowedUncovered = map[string]string{}

// identRe extracts maximal identifier runs, so e.g. "trim_start" is one token
// and never registers "trim" as seen merely by substring containment.
var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// wordSet builds the set of identifier-shaped words appearing in any of the
// given source strings.
func wordSet(sources []string) map[string]bool {
	seen := map[string]bool{}
	for _, s := range sources {
		for _, w := range identRe.FindAllString(s, -1) {
			seen[w] = true
		}
	}
	return seen
}

// uncoveredBuiltins returns the entries of names absent from the word set
// built from wispFiles and testLiterals combined. Pure and independent of
// real repo state, so it is directly unit-testable against synthetic input
// (mirrors editors_test.go's reconcileReserved pattern).
func uncoveredBuiltins(names []string, wispFiles []string, testLiterals []string) []string {
	seen := wordSet(append(append([]string{}, wispFiles...), testLiterals...))
	var out []string
	for _, n := range names {
		if !seen[n] {
			out = append(out, n)
		}
	}
	return out
}

// validateAllowedUncovered rejects any entry whose justification is empty or
// whitespace-only. Factored into its own helper so the non-vacuity subtest
// below can exercise the exact production check, not a re-implementation of
// it that could drift from the real one.
func validateAllowedUncovered(allowed map[string]string) []string {
	var bad []string
	for name, justification := range allowed {
		if strings.TrimSpace(justification) == "" {
			bad = append(bad, name)
		}
	}
	return bad
}

// collectGoldenWispFiles walks testdata/golden recursively and returns the
// whole content of every *.wisp file (including nested *.dir/ module
// fixtures).
func collectGoldenWispFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	root := "../../testdata/golden"
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".wisp" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, string(b))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// collectCodegenTestStringLiterals parses every *_test.go file in this
// package (excluding this gate's own file) and returns the unquoted content
// of every string literal. Scanning only string-literal contents (not raw Go
// text) is what excludes e.g. `map[string]bool` from registering the "map"
// builtin as covered -- that's Go syntax, not a Wisp program.
func collectCodegenTestStringLiterals(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob *_test.go: %v", err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		if filepath.Base(f) == "builtin_coverage_test.go" {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil {
				out = append(out, s)
			} else {
				out = append(out, lit.Value)
			}
			return true
		})
	}
	return out
}

// TestBuiltinDispatch_AllNamesCovered fails if any types.BuiltinNames() entry
// (not present in allowedUncovered) never appears as a whole-word identifier
// in the compiled-test corpus described above.
func TestBuiltinDispatch_AllNamesCovered(t *testing.T) {
	if bad := validateAllowedUncovered(allowedUncovered); len(bad) > 0 {
		t.Errorf("allowedUncovered entries with empty/whitespace-only justification: %v", bad)
	}

	wispFiles := collectGoldenWispFiles(t)
	testLiterals := collectCodegenTestStringLiterals(t)

	var names []string
	for _, n := range types.BuiltinNames() {
		if _, ok := allowedUncovered[n]; ok {
			continue
		}
		names = append(names, n)
	}

	if uncovered := uncoveredBuiltins(names, wispFiles, testLiterals); len(uncovered) > 0 {
		t.Fatalf("builtins with zero compiled-test occurrence (add a fixture/test, or add a "+
			"justified entry to allowedUncovered in internal/codegen/builtin_coverage_test.go): %v",
			uncovered)
	}
}

// TestBuiltinDispatch_HelperDetectsSyntheticGap proves uncoveredBuiltins can
// actually fail: a synthetic name absent from synthetic corpora must come
// back as uncovered. Required so the gate itself is not vacuous.
func TestBuiltinDispatch_HelperDetectsSyntheticGap(t *testing.T) {
	got := uncoveredBuiltins(
		[]string{"zzz_not_a_real_builtin"},
		[]string{"fn main() -> int { print(\"hi\") return 0 }"},
		[]string{"print"},
	)
	if len(got) != 1 || got[0] != "zzz_not_a_real_builtin" {
		t.Fatalf("uncoveredBuiltins = %v, want [zzz_not_a_real_builtin]", got)
	}
}

// TestBuiltinDispatch_EmptyJustificationRejected exercises the SAME
// production validateAllowedUncovered helper against a synthetic
// empty/whitespace justification, proving the anti-gate-weakening
// non-emptiness check is itself enforced, not just documented.
func TestBuiltinDispatch_EmptyJustificationRejected(t *testing.T) {
	got := validateAllowedUncovered(map[string]string{
		"some_builtin":  "   ",
		"other_builtin": "a real reason",
	})
	if len(got) != 1 || got[0] != "some_builtin" {
		t.Fatalf("validateAllowedUncovered = %v, want [some_builtin]", got)
	}
}
