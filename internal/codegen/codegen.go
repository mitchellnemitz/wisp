// Package codegen lowers a type-checked wisp program to a single self-contained
// POSIX-sh script (spec section 9). Generate is the entry point.
//
// # Consuming the checker result
//
// Generate never re-derives types or names from the AST: it reads everything
// from the *types.Info produced by types.Check. Specifically it uses
// Info.Types (expression types, to pick int/string/bool lowerings and to choose
// bool_int vs bool_str), Info.Uses (identifier -> resolved Var, for the mangled
// shell name), Info.Vars (let -> Var), Info.Calls (call -> CallInfo with the
// defaults-filled argument list and the callee's mangled name / builtin id),
// Info.Funcs (func -> FuncInfo with the function's mangled name and the full
// list of locals to declare), and Info.Main (the entry point). Reserved
// constants stdout/stderr appear as *ast.Ident nodes that are not in Info.Uses;
// they are resolved here by name to the literal fd 1/2.
//
// # Calling convention and temporaries
//
// Every value-producing emission writes the global return register __ret, and
// the result is immediately spilled into a fresh, uniquely-named temporary
// (__wisp_t<n>) before the next __ret-writing emission, so evaluation order is
// strict left-to-right and no pending value is clobbered (spec section 9.2).
// Condition predicates spill to __wisp_c<n>. These temp prefixes are distinct
// from the function prefix (__wisp_f_) and the local-variable prefix
// (__wisp_v_), so a temp can never collide with a mangled user name (spec
// section 9.6 invariant 6).
package codegen

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// SourcePos is the source location a generated line maps back to. It is a thin
// wrapper over token.Position; the source map projects it to {l,c} (spec
// section 3.2).
type SourcePos = token.Position

// Generate compiles a type-checked program to a complete POSIX-sh script. prog
// must have been accepted by types.Check (len(info.Errors) == 0); Generate does
// not re-validate. It returns an error only for an internal inconsistency
// (e.g. a missing main), never for user-program errors.
//
// Generate is a thin wrapper over GenerateWithMap that discards the line map.
func Generate(prog *ast.Program, info *types.Info) ([]byte, error) {
	script, _, err := GenerateWithMap(prog, info)
	return script, err
}

// CoverInst is one coverable source position in coverage mode: the (file, line)
// of an executable statement. The set of CoverInst returned by a coverage-mode
// build is the authoritative coverage UNIVERSE (the lines the runner reports
// against), derived from a source-level AST walk over every function and test
// body BEFORE tree-shaking (buildCoverUniverse) -- so a function no test calls
// still appears (reported 0%-covered), not absent. Deduped per (file, line).
type CoverInst struct {
	File string
	Line int
}

// GenerateLinked compiles a type-checked linked multi-module program (M8) to one
// self-contained script. It concatenates every module's declarations (root first,
// then by modid) into one combined program and reuses the single-program emitter;
// names are already modid-mangled in info, struct identity is token-keyed, and the
// reachability tree-shaker drops every unreached (including unused-import)
// function. The output is one .sh, byte-shaped exactly as a single-file build of
// the same reachable code.
func GenerateLinked(linked *module.Linked, info *types.Info) ([]byte, []*SourcePos, error) {
	combined := &ast.Program{}
	for _, m := range linked.Modules {
		combined.Funcs = append(combined.Funcs, m.Prog.Funcs...)
		combined.Structs = append(combined.Structs, m.Prog.Structs...)
	}
	// Test mode is keyed off the ROOT file's name (a `*_test.wisp`). A test file
	// has no user `fn main`; the emitted runner is the main (spec R12). The root's
	// `test (...)` declarations drive the runner; they live only on the root
	// program (the parser produces them only for a `*_test.wisp`), so they are
	// carried over here. A non-test build copies no Tests and emits the ordinary
	// main footer, byte-identical to today.
	root := linked.Modules[0].Prog
	if isTestFile(root.File) {
		combined.File = root.File
		combined.Tests = root.Tests
		script, lm, _, err := generate(combined, info, true, false)
		return script, lm, err
	}
	script, lm, _, err := generate(combined, info, false, false)
	return script, lm, err
}

// GenerateLinkedCoverage is GenerateLinked in coverage mode (spec R15-R17): the
// emitter additionally writes a `__wisp_cov "<file>:<line>"` hit-marker before
// each executable statement and returns the instrumented (file,line) universe.
// The non-coverage entry points are unchanged and byte-identical; coverage is
// the only thing that alters the emitted script.
func GenerateLinkedCoverage(linked *module.Linked, info *types.Info) (script []byte, lineMap []*SourcePos, universe []CoverInst, err error) {
	combined := &ast.Program{}
	for _, m := range linked.Modules {
		combined.Funcs = append(combined.Funcs, m.Prog.Funcs...)
		combined.Structs = append(combined.Structs, m.Prog.Structs...)
	}
	root := linked.Modules[0].Prog
	if isTestFile(root.File) {
		combined.File = root.File
		combined.Tests = root.Tests
		return generate(combined, info, true, true)
	}
	return generate(combined, info, false, true)
}

// isTestFile reports whether file names a `*_test.wisp` test file (the
// test-mode trigger, mirroring the checker's notion). Codegen keeps its own copy
// rather than importing the unexported checker helper.
func isTestFile(file string) bool {
	return strings.HasSuffix(file, "_test.wisp")
}

// GenerateWithMap compiles a type-checked program to a complete POSIX-sh script
// and, alongside it, a per-generated-line source-position table (spec section
// 3.3). lineMap[i] is the wisp source position that produced generated line i+1
// (1-based generated line), or nil when the line has no wisp origin (shebang,
// banner, prelude helper bodies, the trailing footer, inter-function blanks).
// The table has exactly one entry per generated line (a generated line is a
// maximal run terminated by \n; a trailing \n does not add a phantom entry).
//
// The table is built DURING emission by counting the lines each section writes,
// so it cannot drift from the actual output line count (spec section 9, risks).
func GenerateWithMap(prog *ast.Program, info *types.Info) (script []byte, lineMap []*SourcePos, err error) {
	script, lm, _, err := generate(prog, info, false, false)
	return script, lm, err
}

// generate is the shared single-program emitter. testMode selects the runner
// footer (a `*_test.wisp`): the user `fn main` is absent and the emitted runner
// IS the main, so the nil-main guard and the main-call footer are bypassed (spec
// R12 / AC15). A non-test build (testMode == false) requires a valid main and
// emits the ordinary footer, byte-for-byte as before.
func generate(prog *ast.Program, info *types.Info, testMode, coverage bool) (script []byte, lineMap []*SourcePos, universe []CoverInst, err error) {
	if !testMode && info.Main == nil {
		return nil, nil, nil, fmt.Errorf("codegen: program has no valid main function")
	}
	g := &gen{info: info, used: map[string]bool{}, coverage: coverage}
	// Error-handling mode is program-conditional (spec section 5 / 9.1): the
	// pending-error guard scaffolding, the runtime depth/pending/msg state, and
	// the mode-aware __wisp_fail are emitted ONLY when the program contains a try
	// or throw anywhere. A program with neither is byte-for-byte the M4 shape.
	g.errMode = programUsesErrorHandling(prog)
	g.usesWrap = programUsesWrap(prog)

	// In test mode, locate the optional zero-arg `fn setup` / `fn teardown`
	// lifecycle hooks (recognized BY NAME, not reserved -- spec R9). Only a
	// top-level zero-parameter function named setup/teardown qualifies; anything
	// else is ignored. They seed reachability and the runner calls them per test.
	var setupFn, teardownFn *ast.FuncDecl
	if testMode {
		for _, fn := range prog.Funcs {
			if len(fn.Params) != 0 {
				continue
			}
			switch fn.Name {
			case "setup":
				setupFn = fn
			case "teardown":
				teardownFn = fn
			}
		}
	}

	// Tree-shake: emit only functions transitively reachable from the root main
	// (spec acceptance 6). An unused function -- including all functions of an
	// unused import -- is dropped. Dead code is never executed, so behavior is
	// preserved. In test mode there is no main; reachability is seeded from every
	// test body and the lifecycle hooks (spec R12).
	var reach map[*ast.FuncDecl]bool
	if testMode {
		var hooks []*ast.FuncDecl
		if setupFn != nil {
			hooks = append(hooks, setupFn)
		}
		if teardownFn != nil {
			hooks = append(hooks, teardownFn)
		}
		reach = reachableFromTests(info, prog.Tests, hooks)
	} else {
		reach = reachableFuncs(info)
	}

	// Collect monomorphization instances for numeric-bounded functions.
	monoInstances := collectMonoInstances(info)

	// Emit the function bodies first, recording one line-map entry per emitted
	// output line (g.line appends to g.bodyMap). The blank line written between
	// functions has no wisp origin (nil).
	var body strings.Builder
	for _, fn := range prog.Funcs {
		if !reach[fn] {
			continue
		}
		g.out = &body
		if instances, ok := monoInstances[fn]; ok {
			for _, inst := range instances {
				g.typeSubst = inst.typeSubst
				g.genFuncWithName(fn, inst.name)
				g.typeSubst = nil
				body.WriteString("\n")
				g.bodyMap = append(g.bodyMap, nil)
			}
		} else {
			g.genFunc(fn)
			body.WriteString("\n")
			g.bodyMap = append(g.bodyMap, nil)
		}
	}

	// In test mode, emit each `test (...)` block as a zero-arg shell function
	// __wisp_test_<i> (i 1-based, source order). The runner footer calls these.
	// Emitting each test as a function (rather than inline) reuses the full body
	// machinery -- locals, spill temps, the `local` declaration -- exactly as a
	// user function, so a test body behaves like any other code.
	if testMode {
		g.out = &body
		for i, td := range prog.Tests {
			g.genTestFunc(td, testFuncName(i))
			body.WriteString("\n")
			g.bodyMap = append(g.bodyMap, nil)
		}
	}

	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# generated by wisp; do not edit\n")
	// File-wide ShellCheck directives for findings that generated code triggers
	// by construction, not by mistake (spec section 9.1 readability / ShellCheck
	// gate). SC3043: `local` is committed as a target assumption (ash/dash/busybox
	// all support it; recursion depends on it -- spec section 9.2). SC2050: a
	// comparison may have two compile-time-constant operands (a literal predicate
	// or comparison); the test is intentional, not a forgotten `$`. SC2043: the
	// single-iteration `for ... in 0` once-wrapper used to lower a wisp `for` body
	// is meant to run once (spec section 9.4).
	b.WriteString("# shellcheck disable=SC3043,SC2050,SC2043\n")
	// zsh does not word-split an unquoted $var by default, but wisp's array/dict
	// `for x in $list` lowering relies on sh word splitting. Restore it under zsh
	// ONLY -- a no-op under dash/busybox/bash, which never set ZSH_VERSION and so
	// never run the zsh-only emulate/setopt builtins. Runs in the current shell
	// (a subshell would not persist the option); plain `emulate sh` (not -L) so it
	// holds for the rest of the script.
	//
	// zsh also caps function-call nesting at the FUNCNEST parameter (default 700),
	// so deep wisp recursion aborts under zsh ("maximum nested function level
	// reached") while dash/bash bound it only by the OS stack (M5). zsh has no
	// "unlimited" setting -- UNSETTING FUNCNEST reverts to the 700 default, and
	// FUNCNEST=0 means zero nesting (immediate abort), the opposite of bash where 0
	// is unlimited. So raise it to a large fixed ceiling (1000000) under zsh only;
	// past that the zsh process is bounded by the C stack like the other shells
	// (all four crash in the same ~10k-frame ballpark). 1000000 is well past any
	// realistic recursion yet leaves the OS stack as the real backstop. This runs
	// only inside the ZSH_VERSION guard, so bash (where FUNCNEST has the opposite
	// meaning) never sees it. The ceiling is documented in www/src/content/docs/guide/internals.md.
	b.WriteString(`if [ -n "${ZSH_VERSION:-}" ]; then emulate sh 2>/dev/null || setopt shwordsplit; FUNCNEST=1000000; fi` + "\n")
	b.WriteString("\n")
	// Banner: shebang + 2 comment lines + the zsh shim + 1 blank, all no origin.
	lineMap = make([]*SourcePos, 0, 5+len(g.bodyMap)+1)
	for i := 0; i < 5; i++ {
		lineMap = append(lineMap, nil)
	}
	if prelude := runtime.EmitMode(g.usedHelpers(), g.errMode, g.usesWrap); prelude != "" {
		b.WriteString(prelude)
		b.WriteString("\n")
		// Prelude helper bodies plus the trailing blank: all with no wisp origin.
		for i := 0; i < strings.Count(prelude, "\n")+1; i++ {
			lineMap = append(lineMap, nil)
		}
	}
	// Initialize the error-handling runtime state once, before main, when the
	// program uses try/throw (program-conditional; absent otherwise so a non-
	// error program emits no __wisp_try_depth -- the zero-overhead requirement).
	if g.errMode {
		// pending/msg drive the unwind; code and pos travel WITH a suspended error
		// (code -> e.code on catch; pos -> the located "wisp: <pos>: <msg>" abort
		// when a throw escapes uncaught, M4). All four are saved/restored across
		// every try boundary (see genTry) so a nested fault cannot clobber them.
		b.WriteString("__wisp_try_depth=0\n__wisp_err_pending=\n__wisp_err_msg=\n__wisp_err_code=\n__wisp_err_pos=\n")
		lineMap = append(lineMap, nil, nil, nil, nil, nil)
	}
	// program_path(): capture $0 ONCE at TOP LEVEL (before any function runs, in
	// both the testMode and non-testMode paths), so the value is the script path
	// on all four shells regardless of where program_path() is later read (spec
	// P1). $0 inside a function is the function name in zsh's native mode; the
	// top-level capture is robust independent of the banner's `emulate sh` shim.
	// Tree-shaken: emitted ONLY when the Arg0 sentinel was registered by the
	// program_path() lowering, so a program that never calls it is byte-identical
	// to before (spec P2). `used` is fully populated here (bodies are generated
	// before this footer-assembly step). The capture is a double-quoted expansion
	// of $0 into the reserved global, so the value is inert data (spec P5).
	if g.used[runtime.Arg0] {
		b.WriteString("__wisp_arg0=\"$0\"\n")
		lineMap = append(lineMap, nil)
	}
	b.WriteString(body.String())
	lineMap = append(lineMap, g.bodyMap...)
	if testMode {
		// The runner IS the main for a `*_test.wisp`: it iterates the test
		// functions with the per-test lifecycle and prints TAP-13, then exits with
		// the aggregate code. The nil-main footer below is bypassed (spec R12/AC15).
		runner, runnerLines := g.emitRunner(prog.Tests, setupFn, teardownFn)
		b.WriteString(runner)
		for i := 0; i < runnerLines; i++ {
			lineMap = append(lineMap, nil) // the runner footer has no wisp origin
		}
	} else {
		b.WriteString(g.info.Funcs[info.Main].Mangled)
		if info.MainArgs {
			// Forward the script's positional parameters into main so its `"$@"`
			// materialization (spec 4.5) sees the real argv.
			b.WriteString(" \"$@\"")
		}
		b.WriteString("; exit \"$__ret\"\n")
		lineMap = append(lineMap, nil) // footer has no wisp origin
	}
	if coverage {
		universe = buildCoverUniverse(prog)
	}
	return []byte(b.String()), lineMap, universe, nil
}

type gen struct {
	info *types.Info
	out  *strings.Builder

	// bodyMap records one source-position entry per output line written to the
	// function-body builder via g.line (spec section 3.3). curPos is the source
	// position attributed to lines emitted right now; nil means no wisp origin
	// (used for the structural/scaffold lines that have no causing statement).
	bodyMap []*SourcePos
	curPos  *SourcePos

	tmpCount  int
	condCount int
	onceCount int
	indent    int

	// used records which runtime helper IDs the program references, for
	// tree-shaking (spec section 9.1).
	used map[string]bool

	// errMode is true when the program contains a try or throw, enabling the
	// pending-error guard scaffolding (M5). When false no guard is emitted (zero
	// overhead) and the output is the M4 shape.
	errMode bool
	// usesWrap is true when the program calls the `wrap` builtin anywhere (any
	// expression position, any function or test). It gates the __wisp_err_cause
	// throw-path threading: `wrap` is the only producer of a cause, so a program
	// that never calls it can carry no cause and the threading is dead code. The
	// gate keeps any non-`wrap` program byte-identical to before this feature.
	usesWrap bool
	// guardDepth is the number of currently-open `if [ -z "$__wisp_err_pending" ];
	// then` skip-guard blocks (M5). Each guarded statement opens one; each call/
	// value-producer spill point opens a nested one so the rest of the statement's
	// evaluation is skipped on a fault (fail-at-first-fault, mid-statement). They
	// close in LIFO order at the statement (or condition) boundary.
	guardDepth int
	// guardOpenLines is a stack, parallel to the open guards, of the output line
	// count when each guard opened. closeGuardsTo uses it to detect an empty guard
	// body and fill it with a `:` no-op (avoids an empty `then`).
	guardOpenLines []int
	// tryID is a monotonic per-program counter for the unique per-try save-slot
	// variable names (__wisp_sp_<id> etc.).
	tryID int

	// shellDepth is the number of shell loop constructs currently open. loops is
	// the stack of enclosing wisp loops, innermost last.
	shellDepth int
	loops      []loopCtx

	// scopes mirrors the checker's lexical scoping so an assignment target (which
	// the checker does not record in Info.Uses) resolves to the same Var the
	// checker bound it to. Pushed/popped around the same blocks the checker uses.
	scopes []map[string]*types.Var

	// typeSubst is set during monomorphized emission of a numeric-bounded function.
	// Maps type-variable encoding ("$T") to the concrete type (types.Int or types.Float).
	typeSubst map[types.Type]types.Type

	// curFI is the FuncInfo of the function currently being emitted. A tuple-
	// destructuring statement's bound Vars live ONLY in curFI.Decls (they are not
	// keyed by node in any info map), so genTupleBind resolves each slot's Var by
	// its slot position within curFI.Decls.
	curFI *types.FuncInfo

	// coverage enables coverage instrumentation (spec R15-R17): a
	// `__wisp_cov "<file>:<line>"` hit-marker is emitted before each executable
	// statement. Off by default; the only switch that alters the emitted script.
	// The universe is NOT collected here; it is derived from a source-level AST
	// walk (buildCoverUniverse) so untested functions report 0%, not absent.
	coverage bool
}

// emitCoverMarker emits the coverage hit-marker for the current statement
// position. The marker is ONE newline-terminated append-write of
// `<file>:<line>` via __wisp_cov; the <file> is a double-quoted INERT literal
// (never re-evaluated or globbed, N1) and <line> is a compiler integer. No-op
// unless coverage is on and a real source position is current (scaffold lines
// with no wisp origin are skipped). The marker provides runtime HITS only; it
// does NOT feed the coverage universe -- the universe is derived from a
// source-level AST walk (buildCoverUniverse) so a tree-shaken function still
// appears (0%-covered), not absent.
func (g *gen) emitCoverMarker() {
	if !g.coverage || g.curPos == nil {
		return
	}
	g.use(runtime.Cov)
	g.line("__wisp_cov %s", shellDoubleQuote(fmt.Sprintf("%s:%d", g.curPos.File, g.curPos.Line)))
}

// buildCoverUniverse derives the coverage UNIVERSE from the ORIGINAL SOURCE,
// independent of reachability (spec R16): an AST walk over every function body
// AND every `test (...)` body in the combined program, enumerating every
// coverable statement line, deduped per (file,line) in first-seen order. The
// combined program's Funcs/Tests hold ALL functions BEFORE the tree-shaker runs,
// so a function that no caller references still contributes its body lines and
// reports uncovered rather than vanishing. Uncalled generics fall out naturally
// (their source lines are enumerated; there is no instantiation to run).
func buildCoverUniverse(prog *ast.Program) []CoverInst {
	seen := map[CoverInst]bool{}
	var universe []CoverInst
	add := func(p token.Position) {
		inst := CoverInst{File: p.File, Line: p.Line}
		if !seen[inst] {
			seen[inst] = true
			universe = append(universe, inst)
		}
	}
	for _, fn := range prog.Funcs {
		walkCoverableStmts(fn.Body, add)
	}
	for _, td := range prog.Tests {
		walkCoverableStmts(td.Body, add)
	}
	return universe
}

// loopCtx records, for one enclosing wisp loop, how a wisp break/continue maps
// to shell break counts. See the for-loop lowering note in genFor.
type loopCtx struct {
	isFor bool
}

func (g *gen) use(id string) { g.used[id] = true }

func (g *gen) usedHelpers() []string {
	out := make([]string, 0, len(g.used))
	for _, id := range runtime.IDs() { // stable order
		if g.used[id] {
			out = append(out, id)
		}
	}
	return out
}

func (g *gen) newTemp() string {
	g.tmpCount++
	return fmt.Sprintf("__wisp_t%d", g.tmpCount)
}

func (g *gen) newCond() string {
	g.condCount++
	return fmt.Sprintf("__wisp_c%d", g.condCount)
}

// line writes one indented line of shell and records its source-position entry
// in bodyMap (the current curPos). Every g.line call emits exactly one
// \n-terminated line, so the map stays aligned 1:1 with the output.
func (g *gen) line(format string, args ...any) {
	for i := 0; i < g.indent; i++ {
		g.out.WriteString("\t")
	}
	if len(args) == 0 {
		g.out.WriteString(format)
	} else {
		fmt.Fprintf(g.out, format, args...)
	}
	g.out.WriteString("\n")
	g.bodyMap = append(g.bodyMap, g.curPos)
}

func (g *gen) genFunc(fn *ast.FuncDecl) {
	g.genFuncWithName(fn, g.info.Funcs[fn].Mangled)
}

func (g *gen) genFuncWithName(fn *ast.FuncDecl, name string) {
	fi := g.info.Funcs[fn]
	g.curFI = fi
	defer func() { g.curFI = nil }()
	// Per-function source comment (spec section 9.1 / M2 forward-compat): carry
	// the function's source position into the output. The comment, signature,
	// local declarations, and closing brace attribute to the function position;
	// each statement inside overrides curPos to its own position.
	pos := fn.Pos()
	g.curPos = &pos
	g.line("# %s:%d", pos.File, pos.Line)
	g.line("%s() {", name)
	g.indent++

	// Reset per-function temp counters so names are stable and small.
	g.tmpCount = 0
	g.condCount = 0
	g.onceCount = 0
	g.shellDepth = 0
	g.loops = nil

	// Parameter scope (scopes[0]); params are declared here.
	g.scopes = []map[string]*types.Var{{}}
	for _, d := range fi.Decls {
		if d.IsParam {
			g.declareVar(d)
		}
	}

	// Generate the parameter copy-in and the body into a buffer first, so we know
	// exactly which spill temporaries (__wisp_t/__wisp_c/__wisp_once) the function
	// uses. They are then declared `local` alongside the params: without this a
	// callee, which reuses the same global temp names (the counter resets per
	// function), would clobber a caller's still-live temp -- silently corrupting
	// recursion and any `call() OP call()` expression.
	// `fn main(args: string[])` materializes "$@" into an array handle (genMainArgs).
	mainArgs := fn == g.info.Main && g.info.MainArgs
	saved := g.out
	var body strings.Builder
	g.out = &body
	bodyStart := len(g.bodyMap)
	g.copyParams(fi, fn, mainArgs)
	g.genBlock(fn.Body)
	g.out = saved

	// g.line appended one bodyMap position per body line during the buffered
	// pass. The `local` line is emitted AFTER the body was generated (we needed
	// the final temp counts) but appears BEFORE the body in the output, so move
	// the body's positions back after the local-line position to keep bodyMap in
	// output-line order (the source map depends on this lockstep).
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

// emitLocalDecl declares, with `local`, every name a function frame owns: its
// parameters and block lets (fi.Decls), plus the spill temporaries it actually
// used (__wisp_t<n>, __wisp_c<n>, __wisp_once<n>). Declaring the temps local is
// what isolates each call frame so a callee cannot clobber a caller's live temp.
// Must be called AFTER the body is generated so the counters are final.
func (g *gen) emitLocalDecl(fi *types.FuncInfo) {
	names := make([]string, 0, len(fi.Decls)+g.tmpCount+g.condCount+g.onceCount)
	for _, d := range fi.Decls {
		names = append(names, d.Mangled)
	}
	for i := 1; i <= g.tmpCount; i++ {
		names = append(names, fmt.Sprintf("__wisp_t%d", i))
	}
	for i := 1; i <= g.condCount; i++ {
		names = append(names, fmt.Sprintf("__wisp_c%d", i))
	}
	for i := 1; i <= g.onceCount; i++ {
		names = append(names, fmt.Sprintf("__wisp_once%d", i))
	}
	if len(names) > 0 {
		g.line("local %s", strings.Join(names, " "))
	}
}

// resolveType resolves a type through the current monomorphization substitution.
// Returns t unchanged when not inside a monomorphized emission context.
func (g *gen) resolveType(t types.Type) types.Type {
	if len(g.typeSubst) == 0 {
		return t
	}
	if concrete, ok := g.typeSubst[t]; ok {
		return concrete
	}
	return t
}

// instantiatedCallName returns the shell function name for a user call,
// appending the monomorphization suffix for numeric-bounded type parameters.
func (g *gen) instantiatedCallName(ci *types.CallInfo) string {
	if len(ci.TypeSubst) == 0 {
		return ci.Mangled
	}
	var sb strings.Builder
	sb.WriteString(ci.Mangled)
	for _, tp := range ci.Func.TypeParams {
		if ci.Func.TypeParamBounds[tp] != "numeric" {
			continue
		}
		v, ok := ci.TypeSubst[tp]
		if !ok {
			continue
		}
		concrete := g.resolveType(v)
		sb.WriteString("__")
		sb.WriteString(string(concrete))
	}
	return sb.String()
}

// copyParams copies positional arguments into their mangled parameter names at
// function entry. Parameters and block lets share FuncInfo.Decls (params first);
// the `local` declaration itself is emitted separately by emitLocalDecl.
//
// For `fn main(args: string[])` (mainArgs), the single parameter is NOT copied
// from $1; instead the positional parameters "$@" are materialized into a fresh
// array handle so `args` is an ordinary string[] (spec 4.5).
func (g *gen) copyParams(fi *types.FuncInfo, fn *ast.FuncDecl, mainArgs bool) {
	// Copy positional parameters into their mangled names. Walk the source
	// parameter list so blank params advance the positional slot without binding;
	// named params resolve their Decl in fi.Decls (params-first, same order).
	declIdx := 0
	for slot := range fn.Params {
		p := &fn.Params[slot]
		if p.Name == "_" {
			continue // consumes slot (slot+1) but binds nothing
		}
		d := fi.Decls[declIdx]
		declIdx++
		if mainArgs {
			// The sole main(args) parameter is materialized below, not copied.
			g.genMainArgs(d.Mangled)
			continue
		}
		g.line("%s=\"$%d\"", d.Mangled, slot+1)
	}
}

// genMainArgs materializes the script's positional parameters "$@" into a fresh
// array handle assigned to dst (the `args` parameter's mangled name). Each
// positional is copied into __wisp_a_<id>_<i> (0-based) and _len is set, so
// length/index/for-in/push all behave uniformly (spec 4.5).
func (g *gen) genMainArgs(dst string) {
	g.use(runtime.Alloc)
	g.line("__wisp_alloc")
	g.line("%s=\"$__ret\"", dst)
	g.line("__wisp_argc=0")
	g.line("for __wisp_arg in \"$@\"; do")
	g.indent++
	g.line("eval \"__wisp_a_${%s}_${__wisp_argc}=\\$__wisp_arg\"", dst)
	g.line("__wisp_argc=$(( __wisp_argc + 1 ))")
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$__wisp_argc\"", dst)
}

func (g *gen) genBlock(stmts []ast.Stmt) {
	for _, s := range stmts {
		g.genStmt(s)
	}
}

// --- scope tracking (mirrors the checker, for assignment-target resolution) ---

func (g *gen) pushScope() { g.scopes = append(g.scopes, map[string]*types.Var{}) }
func (g *gen) popScope()  { g.scopes = g.scopes[:len(g.scopes)-1] }

func (g *gen) declareVar(v *types.Var) {
	g.scopes[len(g.scopes)-1][v.Name] = v
}

// resolveVar finds the innermost binding of name, matching the checker's
// lexical lookup.
func (g *gen) resolveVar(name string) *types.Var {
	for i := len(g.scopes) - 1; i >= 0; i-- {
		if v, ok := g.scopes[i][name]; ok {
			return v
		}
	}
	return nil
}
