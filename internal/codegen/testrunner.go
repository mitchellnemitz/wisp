package codegen

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// The test-mode runner codegen (spec R9/R11/R12). When the driver compiles a
// `*_test.wisp`, generate() emits, instead of a user `fn main`, a self-contained
// runner: each `test (...)` block becomes a zero-arg shell function
// __wisp_test_<i>, and a runner footer iterates them with the per-test lifecycle
// (tmpdir -> setup -> body -> teardown -> rm) and prints TAP-13 to stdout.
//
// Result discrimination is by the per-test PROCESS subshell's exit code
// (consumed from $?, NOT the in-shell __ret return register): 0 = pass, 121 =
// SKIP, 122 = assertion FAIL, any other nonzero = unexpected ERROR (spec
// Execution model; the codes are produced by __wisp_assert_fail / __wisp_skip,
// Task 2). The test's stderr is captured to a per-test temp file so the runner
// can put the located diagnostic in the TAP output WITHOUT disturbing the exit
// code: a `( ... ) 2>FILE` redirection leaves $? equal to the subshell's real
// exit on all four shells (no command substitution, no pipeline -- both of which
// would replace $? with the substitution/last-element status).
//
// Injection-safety (N1): the test name is a compile-time string literal emitted
// as a single-quoted shell literal and printed only via `printf '%s'`; the
// captured diagnostic and skip reason reach output only through `printf '%s'`
// with the value in a double-quoted expansion. Nothing is re-evaluated or
// globbed.

// testFuncName is the shell function name for the i-th (0-based) test block.
func testFuncName(i int) string {
	return fmt.Sprintf("__wisp_test_%d", i+1)
}

// escapeTAPDesc escapes a test name for emission into a TAP-13 result-line
// DESCRIPTION. The CLI TAP parser treats an unescaped `#` as the start of a
// SKIP/TODO directive, so a name containing `#` would be misparsed as a
// directive (a passing test shown as skipped, a failing test's diagnostic
// lost). Per TAP-13, escape `\` as `\\` and `#` as `\#` (backslash FIRST, so a
// literal `\` is not later confused with the escape we introduce). The runner's
// own directive lines stay UNescaped, so a directive is recognized only when it
// is real. The parser reverses this before storing the name, so it round-trips.
//
// The name is a compile-time string literal, so this runs at codegen time on a
// constant; nothing is evaluated at runtime and the result is still emitted as
// an inert single-quoted literal printed via `printf '%s'`.
func escapeTAPDesc(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "#", `\#`)
	return s
}

// genTestFunc emits one `test (...)` block as a zero-arg shell function. It
// mirrors genFuncWithName for a no-parameter `-> void` body, reading the test's
// locals/spill-temp declarations from info.Tests (the checker recorded them
// there; a test is not in info.Funcs because it is not callable).
func (g *gen) genTestFunc(td *ast.TestDecl, name string) {
	fi := g.info.Tests[td]
	pos := td.Pos()
	g.curPos = &pos
	g.line("# %s:%d", pos.File, pos.Line)
	g.line("%s() {", name)
	g.indent++

	// Reset per-function counters so spill-temp names are stable and small.
	g.tmpCount = 0
	g.condCount = 0
	g.onceCount = 0
	g.shellDepth = 0
	g.loops = nil

	// A test body has no parameters; its only frame-owned names are block lets and
	// the spill temps it uses. Buffer the body first so the final temp counts are
	// known before the `local` declaration is emitted (same ordering dance as
	// genFuncWithName, keeping bodyMap aligned to output lines).
	g.scopes = []map[string]*types.Var{}
	g.pushScope()
	saved := g.out
	var body strings.Builder
	g.out = &body
	bodyStart := len(g.bodyMap)
	g.genBlock(td.Body)
	g.out = saved

	bodyPos := append([]*SourcePos(nil), g.bodyMap[bodyStart:]...)
	g.bodyMap = g.bodyMap[:bodyStart]
	g.emitLocalDecl(fi)
	g.out.WriteString(body.String())
	g.bodyMap = append(g.bodyMap, bodyPos...)

	g.scopes = nil
	g.indent--
	g.curPos = &pos
	g.line("}")
	g.curPos = nil
}

// genTestTmpdir lowers test_tmpdir() -> string: it reads the runner-set per-test
// directory variable __wisp_ttmp into the return register. The runner creates a
// fresh __wisp_ttmp before each test and removes it after (after teardown), so
// the value is valid throughout setup, the body, and teardown.
func (g *gen) genTestTmpdir() atom {
	t := g.newTemp()
	g.line(`%s="$__wisp_ttmp"`, t)
	return varAtom(t)
}

// emitRunner writes the runner footer for a `*_test.wisp` and returns the text
// plus its line count (every runner line has no wisp origin in the source map).
// It is the emitted "main": it prints the TAP plan, runs each test through the
// lifecycle, classifies the per-test exit code into a TAP line, and exits
// nonzero iff any test FAILED or ERRORED (skips/passes do not fail the runner).
//
// Lifecycle per test (spec Execution model), all inside ONE per-test process
// subshell so state never leaks between tests (isolation, like bats):
//
//	__wisp_ttmp = a fresh mktemp -d   (the test_tmpdir())
//	setup()                            if a zero-arg `fn setup` exists
//	( body )                           inner subshell: assert_fail/skip `exit`
//	                                   escape ONLY this, so teardown still runs
//	__wisp_tcode=$?                    the body's real exit (0/121/122/other)
//	teardown()                         if a zero-arg `fn teardown` exists; ALWAYS
//	rm -rf "$__wisp_ttmp"              after teardown can still see it
//	exit "$__wisp_tcode"               the per-test process reports the body code
//
// The whole process subshell's stderr is redirected to a per-test file so the
// FAIL/ERROR diagnostic is captured; the redirection does not perturb $?.
func (g *gen) emitRunner(tests []*ast.TestDecl, setupFn, teardownFn *ast.FuncDecl) (string, int) {
	var b strings.Builder
	n := 0
	// Every line goes through Fprintf so `%%` is consistently unescaped to a single
	// `%` (a shell printf format specifier). Lines with no Go substitution still
	// pass their format verbs (`%%`) through Fprintf, so a no-arg line and an
	// arg-bearing line render `%` identically. Bare `%` never appears here.
	w := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteString("\n")
		n++
	}

	w("# --- wisp test runner (TAP version 13) ---")
	// Entry function for the runner. Must NOT collide with any builtin helper
	// emitted into the same script: a test that calls run() pulls in the run()
	// builtin helper (runtime.Run, "__wisp_run"); naming the runner entry the
	// same shadowed that helper and made run() re-enter the whole suite, an
	// unbounded recursion (fork bomb).
	w("__wisp_test_main() {")
	w("\tprintf 'TAP version 13\\n'")
	w("\tprintf '1..%d\\n'", len(tests))
	w("\t__wisp_failed=0")
	// One stderr-capture file reused across tests; created once, removed at the end.
	w("\t__wisp_diag=\"$(mktemp \"${TMPDIR:-/tmp}/wisp_diag.XXXXXX\")\" || { printf 'wisp: test runner: mktemp failed\\n' >&2; exit 1; }")

	for i, td := range tests {
		idx := i + 1
		nameLit := shellSingleQuote(escapeTAPDesc(td.Name))
		tfn := testFuncName(i)
		w("\t# test %d", idx)
		// Per-test process subshell. Its stderr is captured to __wisp_diag; the
		// redirection does not change $? (no command-substitution / no pipeline).
		w("\t(")
		// Per-test tmpdir, created fresh; failure to create is a runner error.
		w("\t\t__wisp_ttmp=\"$(mktemp -d \"${TMPDIR:-/tmp}/wisp_t.XXXXXX\")\" || exit 120")
		if setupFn != nil {
			w("\t\t%s", g.info.Funcs[setupFn].Mangled)
		}
		// Inner subshell holds the body: assert_fail/skip `exit` escapes ONLY it,
		// leaving teardown + rm to run in the per-test process.
		w("\t\t(")
		w("\t\t\t%s", tfn)
		w("\t\t)")
		w("\t\t__wisp_tcode=$?")
		if teardownFn != nil {
			w("\t\t%s", g.info.Funcs[teardownFn].Mangled)
		}
		w("\t\trm -rf \"$__wisp_ttmp\"")
		w("\t\texit \"$__wisp_tcode\"")
		w("\t) 2>\"$__wisp_diag\"")
		w("\t__wisp_outcome=$?")
		// Classify. The name is an inert single-quoted literal printed via %s; the
		// captured diagnostic (read from the file) is printed via %s too, so a
		// metacharacter-laden name/reason/message renders literally (N1).
		w("\tif [ \"$__wisp_outcome\" -eq 0 ]; then")
		w("\t\tprintf 'ok %d - %%s\\n' %s", idx, nameLit)
		w("\telif [ \"$__wisp_outcome\" -eq 121 ]; then")
		// SKIP: stderr shape `<pos>: SKIP: <reason>`; take the text after `SKIP: `.
		w("\t\t__wisp_reason=\"$(__wisp_skip_reason)\"")
		w("\t\tprintf 'ok %d - %%s # SKIP %%s\\n' %s \"$__wisp_reason\"", idx, nameLit)
		w("\telif [ \"$__wisp_outcome\" -eq 122 ]; then")
		// Assertion FAIL: emit `not ok` + a TAP diagnostic carrying the located msg.
		w("\t\t__wisp_failed=1")
		w("\t\tprintf 'not ok %d - %%s\\n' %s", idx, nameLit)
		w("\t\t__wisp_diag_block")
		w("\telse")
		// Any other nonzero: an unexpected ERROR (distinct from an assertion FAIL).
		w("\t\t__wisp_failed=1")
		w("\t\tprintf 'not ok %d - %%s\\n' %s", idx, nameLit)
		w("\t\t__wisp_err_block")
		w("\tfi")
	}

	w("\trm -f \"$__wisp_diag\"")
	w("\tif [ \"$__wisp_failed\" -ne 0 ]; then exit 1; fi")
	w("\texit 0")
	w("}")

	// __wisp_skip_reason: print the reason from the captured stderr. The skip
	// helper wrote `<pos>: SKIP: <reason>`; strip everything up to the first
	// `SKIP: `. Read the file once into a variable; value flows only as data.
	w("__wisp_skip_reason() {")
	w("\t__wisp_r=\"$(cat \"$__wisp_diag\")\"")
	// Invariant: nothing prints `SKIP: ` to stderr before the skip helper's own
	// `<pos>: SKIP: <reason>` line, so stripping up to the FIRST `SKIP: ` yields
	// exactly the reason. Reasons are single-line compile-time string literals.
	w("\t__wisp_r=\"${__wisp_r#*SKIP: }\"")
	w("\tprintf '%%s' \"$__wisp_r\"")
	w("}")

	// __wisp_diag_block: render the captured stderr as a TAP `#`-comment
	// diagnostic, one `# ` per line. Each line is printed via %s (inert).
	w("__wisp_diag_block() {")
	w("\twhile IFS= read -r __wisp_line; do")
	w("\t\tprintf '# %%s\\n' \"$__wisp_line\"")
	w("\tdone < \"$__wisp_diag\"")
	w("}")

	// __wisp_err_block: like __wisp_diag_block but marks the diagnostic as ERROR
	// (an unexpected fault), so it is visibly distinct from an assertion FAIL.
	w("__wisp_err_block() {")
	w("\tprintf '# ERROR (unexpected fault)\\n'")
	w("\twhile IFS= read -r __wisp_line; do")
	w("\t\tprintf '# %%s\\n' \"$__wisp_line\"")
	w("\tdone < \"$__wisp_diag\"")
	w("}")

	w("__wisp_test_main")
	return b.String(), n
}
