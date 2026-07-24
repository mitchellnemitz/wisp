package docslint_test

import (
	"os"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/docslint"
	"github.com/mitchellnemitz/wisp/internal/types"
)

const (
	stdlibIndexPath = "../../www/src/content/docs/stdlib-index.md"
	languageMdPath  = "../../www/src/content/docs/guide/language.md"
)

func TestStdlibIndexNoStaleBareSpelling(t *testing.T) {
	data, err := os.ReadFile(stdlibIndexPath)
	if err != nil {
		t.Fatalf("reading %s: %v", stdlibIndexPath, err)
	}
	offenses := docslint.StaleBareSpellings(string(data), types.RemovableBuiltins())
	for _, o := range offenses {
		t.Errorf("%s:%d: stale bare spelling %q; use its RemovedHint()-qualified form", stdlibIndexPath, o.Line, o.Name)
	}
}

func TestLanguageMdFuncrefClassesAccurate(t *testing.T) {
	data, err := os.ReadFile(languageMdPath)
	if err != nil {
		t.Fatalf("reading %s: %v", languageMdPath, err)
	}
	mono, overloaded, generic, err := docslint.LanguageMdFuncrefExamples(string(data))
	if err != nil {
		t.Fatalf("extracting funcref examples: %v", err)
	}

	generatable := types.GeneratableBuiltinFuncrefs()
	for _, name := range mono {
		if !generatable[name] {
			t.Errorf("language.md lists %q as monomorphic-generatable, but it is not in GeneratableBuiltinFuncrefs()", name)
		}
	}

	overloadedSet := map[string]bool{}
	for _, n := range types.OverloadedFuncrefNames() {
		overloadedSet[n] = true
	}
	for _, name := range overloaded {
		if !overloadedSet[name] {
			t.Errorf("language.md lists %q as overloaded, but it is not in OverloadedFuncrefNames()", name)
		}
	}

	genericSet := map[string]bool{}
	for _, n := range types.GenericFuncrefNames() {
		genericSet[n] = true
	}
	for _, name := range generic {
		if !genericSet[name] {
			t.Errorf("language.md lists %q as generic, but it is not in GenericFuncrefNames()", name)
		}
	}
}

func TestStdlibIndexComplete(t *testing.T) {
	data, err := os.ReadFile(stdlibIndexPath)
	if err != nil {
		t.Fatalf("reading %s: %v", stdlibIndexPath, err)
	}
	entries := docslint.ParseDocMembers(string(data))
	members := types.DocumentableMembers()
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	missing := docslint.Completeness(entries, ids)
	for _, id := range missing {
		// FR-010 (completeness case: no line applicable): id + expected + actual.
		t.Errorf("%s: catalog member %q: expected a documented entry, actual none (undocumented)", stdlibIndexPath, id)
	}
}

func TestCompleteness_ExecutableNegative(t *testing.T) {
	doc := "## Arrays\n\n- `[x] array.is_empty(xs: T[]) -> bool`\n"
	entries := docslint.ParseDocMembers(doc)
	missing := docslint.Completeness(entries, []string{"array.is_empty", "array.push"})
	if len(missing) != 1 || missing[0] != "array.push" {
		t.Fatalf("Completeness() = %v, want exactly [\"array.push\"]", missing)
	}
}

func TestStdlibIndexParseable(t *testing.T) {
	data, err := os.ReadFile(stdlibIndexPath)
	if err != nil {
		t.Fatalf("reading %s: %v", stdlibIndexPath, err)
	}
	entries := docslint.ParseDocMembers(string(data))
	for _, v := range docslint.ParseViolations(entries, stdlibIndexPath) {
		t.Error(v)
	}
}

func TestParseViolations_ExecutableNegative(t *testing.T) {
	// A checkbox-bearing member bullet whose signature does not parse (a
	// leading-digit "identifier"), a legitimate entry documented twice, and a
	// non-checkbox backtick bullet that must be IGNORED (prose/example, not a
	// member). Expect exactly two violations: one malformed, one duplicate.
	doc := "## Arrays\n\n" +
		"- `[x] array.push(xs: T[], v: T) -> T[]`\n" +
		"- `[x] array.push(xs: T[], v: T) -> T[]`\n" + // duplicate
		"- `[x] 9bad(x: int) -> int`\n" + // malformed: id can't start with a digit
		"- `map(o, f)` -- a bare combinator example, not a member entry\n" // no checkbox: ignored
	entries := docslint.ParseDocMembers(doc)
	got := docslint.ParseViolations(entries, "synthetic.md")
	if len(got) != 2 {
		t.Fatalf("ParseViolations() = %d violations, want 2 (one malformed, one duplicate); got %v", len(got), got)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "9bad") || !strings.Contains(joined, "duplicate") {
		t.Fatalf("ParseViolations() = %v, want one malformed (9bad) and one duplicate", got)
	}
}

// TestParseDocMembers_KeywordLineSkipped guards stdlib-index.md's
// "[x] throw <error>" bullet (documenting the throw KEYWORD's pseudo-syntax,
// not a DocumentableMembers() id): it must be skipped as non-member, never
// reported as a Malformed entry or a completeness/parse violation.
func TestParseDocMembers_KeywordLineSkipped(t *testing.T) {
	doc := "## Errors\n\n" +
		"- `[x] throw <error>` (keyword) / `[x] try { } catch (e) { } finally { }`\n"
	entries := docslint.ParseDocMembers(doc)
	if len(entries) != 0 {
		t.Fatalf("ParseDocMembers() = %v, want no entries (keyword pseudo-syntax line must be skipped)", entries)
	}
	if v := docslint.ParseViolations(entries, "synthetic.md"); len(v) != 0 {
		t.Fatalf("ParseViolations() = %v, want none for a skipped keyword line", v)
	}
}

func TestDocslintGuard_ExecutableNegative(t *testing.T) {
	syntheticDoc := "## Arrays\n\n- `[x] push(a: T[], v: T) -> T[]`\n"
	offenses := docslint.StaleBareSpellings(syntheticDoc, types.RemovableBuiltins())
	found := false
	for _, o := range offenses {
		if o.Name == "push" && o.Line == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("StaleBareSpellings did not detect synthetic drift for %q; got %v", "push", offenses)
	}
}
