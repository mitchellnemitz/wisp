package types

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// delegateFuncLiteral pairs a Group-B function with the flat literal it used to
// hardcode into its own diagnostics (never CallInfo.Builtin, which is excluded
// by name below). Group-A functions (checkMinMaxCall, checkFindAnyAllCall,
// checkFirstLastCall, checkTakeDropCall) are excluded: they never hardcoded a
// literal, they used their own `name` parameter, so there is no literal to
// residually scan for.
//
// Deliberately no line numbers here: Task 1's checkBuiltinNamed doc-comment
// rewrite shifts every line below it in call.go by a few lines, and any other
// future edit shifts lines the same way — hardcoding ranges here would make
// this test fail on drift unrelated to the property it checks. Instead the
// test below locates each function by its signature text and finds its own
// closing brace, so it is immune to line-number drift from unrelated edits.
var delegateFuncLiteral = map[string]struct {
	file    string
	literal string
}{
	"checkAbsCall":          {"stdlib.go", "abs"},
	"checkJoinCall":         {"stdlib.go", "join"},
	"checkContainsCall":     {"stdlib.go", "contains"},
	"checkIndexOfCall":      {"collections.go", "index_of"},
	"checkReverseCall":      {"stdlib.go", "reverse"},
	"checkReduceCall":       {"stdlib.go", "reduce"},
	"checkRunCall":          {"stdlib.go", "run"},
	"checkRunEnvCall":       {"stdlib.go", "run_env"},
	"checkRunEnvStatusCall": {"stdlib.go", "run_env_status"},
	"checkRunEnvFullCall":   {"stdlib.go", "run_env_full"},
	"checkRunStatusCall":    {"stdlib.go", "run_status"},
	"checkRunFullCall":      {"stdlib.go", "run_full"},
	"checkRunInputCall":     {"stdlib.go", "run_input"},
	"checkRunInputFullCall": {"stdlib.go", "run_input_full"},
	"checkExecCommandCall":  {"stdlib.go", "exec_command"},
	"checkSortCall":         {"collections.go", "sort"},
	"checkSortByCall":       {"collections.go", "sort_by"},
	"checkSliceCall":        {"collections.go", "slice"},
	"checkConcatCall":       {"collections.go", "concat"},
	"checkSumCall":          {"collections.go", "sum"},
	"checkRangeCall":        {"collections.go", "range"},
	"checkValuesCall":       {"collections.go", "values"},
	"checkGetOrCall":        {"collections.go", "get_or"},
	"checkGetCall":          {"collections.go", "get"},
	"checkRemoveCall":       {"collections.go", "remove"},
	"checkMergeCall":        {"collections.go", "merge"},
	"checkClampCall":        {"collections.go", "clamp"},
	"checkSignCall":         {"collections.go", "sign"},
	"checkCountWhereCall":   {"collections.go", "count_where"},
	"checkFlattenCall":      {"collections.go", "flatten"},
	"checkUniqueCall":       {"collections.go", "unique"},
	"checkPopCall":          {"collections.go", "pop"},
	"checkRemoveAtCall":     {"collections.go", "remove_at"},
	"checkInsertAtCall":     {"collections.go", "insert_at"},
	"checkSizeCall":         {"collections.go", "size"},
	"checkClearCall":        {"collections.go", "clear"},
	"checkPipeCall":         {"stdlib.go", "pipe"},
	"checkSpawnCall":        {"stdlib.go", "spawn"},
	"checkWaitCall":         {"stdlib.go", "wait"},
	"checkIsDoneCall":       {"stdlib.go", "is_done"},
	"checkSignalCall":       {"stdlib.go", "signal"},
	"checkWaitAnyCall":      {"stdlib.go", "wait_any"},
	"checkMakeFifoCall":     {"stdlib.go", "make_fifo"},
	"checkPushCall":         {"call.go", "push"},
	"checkHasCall":          {"call.go", "has"},
	"checkKeysCall":         {"call.go", "keys"},
	"checkMapCall":          {"call.go", "map"},
	"checkFilterCall":       {"call.go", "filter"},
	"checkEachCall":         {"call.go", "each"},
	"checkZipCall":          {"call.go", "zip"},
}

// legitNonResidual lists the exact (trimmed) source lines where the scanned
// literal appears as something OTHER than a hardcoded builtin-name diagnostic,
// verified correct by hand. The broad regex flags them as false positives.
// Keyed by trimmed line text so editing any line re-triggers the check.
var legitNonResidual = map[string]string{
	`if c.rejectTypeVar(n.Args[0].Pos(), et, "sort") {`:                                                      "checkSortCall: 3rd arg is rejectTypeVar's operation-category label, not a diagnostic builtin name; naming diagnostics use dispName",
	`if c.rejectTypeVar(n.Args[0].Pos(), et, "sum") {`:                                                       "checkSumCall: same rejectTypeVar category-label arg",
	`c.errf(n.Args[1].Pos(), "signal: signal name must be a string literal")`:                                "checkSignalCall: 'signal:' is a message-category prefix; builtin-naming diagnostics already use dispName",
	`c.errf(n.Args[1].Pos(), "signal: unsupported signal: %s", sig)`:                                         "checkSignalCall: same 'signal:' category prefix",
	`c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the dict key type)", dispName, at, kt)`: "checkHasCall: regex hit the English word 'has'; builtin name is the dispName %s",
}

// TestNoResidualFlatLiteral asserts that none of the 50 Group-B functions
// listed above still embed their own dead flat-name literal in a diagnostic
// after the dispName refactor. Each function is located dynamically by its
// signature (immune to line-number drift from unrelated edits elsewhere in
// the same file, e.g. Task 1's checkBuiltinNamed doc-comment rewrite shifting
// every call.go line below it) rather than by a hardcoded line range.
func TestNoResidualFlatLiteral(t *testing.T) {
	for fn, spec := range delegateFuncLiteral {
		src, err := os.ReadFile(spec.file)
		if err != nil {
			t.Fatalf("%s: %v", fn, err)
		}
		lines := strings.Split(string(src), "\n")
		sigPrefix := "func (c *checker) " + fn + "("
		start := -1
		for i, l := range lines {
			if strings.HasPrefix(l, sigPrefix) {
				start = i
				break
			}
		}
		if start == -1 {
			t.Fatalf("%s: signature %q not found in %s (function renamed?)", fn, sigPrefix, spec.file)
		}
		end := -1
		for i := start; i < len(lines); i++ {
			if lines[i] == "}" {
				end = i
				break
			}
		}
		if end == -1 {
			t.Fatalf("%s: no top-level closing brace found after line %d in %s", fn, start+1, spec.file)
		}
		// Match the literal as a whole word anywhere inside a double-quoted Go
		// string literal, not just before a fixed suffix set. A narrower
		// suffix-anchored regex (e.g. matching only "(): "/" expects"/" must")
		// misses real shapes present in this codebase, such as checkMapCall's
		// "map over %s: fn parameter type is %s, want %s" (call.go:1261) and
		// checkFilterCall's "filter is not defined over Result (use and_then)"
		// (call.go:1337) — caught during plan review, Testability lens round 1.
		re := regexp.MustCompile(`"[^"]*\b` + regexp.QuoteMeta(spec.literal) + `\b[^"]*"`)
		for _, line := range lines[start : end+1] {
			if strings.Contains(line, "Builtin:") {
				continue // CallInfo.Builtin must keep the flat literal — not a residual diagnostic
			}
			if re.MatchString(line) {
				if _, ok := legitNonResidual[strings.TrimSpace(line)]; ok {
					continue
				}
				t.Errorf("%s: residual flat-literal diagnostic found: %q", fn, strings.TrimSpace(line))
			}
		}
	}
}
