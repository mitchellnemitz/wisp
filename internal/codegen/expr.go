package codegen

import (
	"fmt"
	"math"
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// atom names a computed value. When lit is true, name is a self-contained safe
// token (int digits, the literal true/false, or a single-quoted string literal)
// usable directly as a word. Otherwise name is a shell variable holding the
// value (a temp, a mangled local, or a reserved-constant fd), expanded as
// "$name" where a word is needed.
type atom struct {
	name string
	lit  bool
}

func litAtom(tok string) atom  { return atom{name: tok, lit: true} }
func varAtom(name string) atom { return atom{name: name} }

// word renders a as a shell word: a quoted expansion for a variable, or the
// safe literal token itself. Every variable use is double-quoted (spec section
// 9.6 invariant 3).
func (g *gen) word(a atom) string {
	if a.lit {
		return a.name
	}
	return "\"$" + a.name + "\""
}

// arith renders a as an operand inside $(( )). Int literals are digit strings;
// variables are referenced BARE as name, not $name (see the variable branch for
// why: $name re-lexes an INT_MIN value and is dash-wrong). Both are injection-safe
// because every int-typed value is [+-]?[0-9]+ (invariant 5).
//
// INT_MIN exception: the token -9223372036854775808 has magnitude 2^63, one past
// INT_MAX; emitting it bare inside $(( )) is mis-tokenized by dash (off-by-one)
// and zsh. Spill it to a temp as a plain assignment and reference the temp BARE
// (no leading $): inside $(( )) a bare name reads the variable's stored value,
// which dash/bash/busybox evaluate correctly. The dollar form $t would
// string-expand to the literal -9223372036854775808 and re-lex that 2^63 token,
// reproducing the dash off-by-one -- so the bare return is mandatory, not
// stylistic (and is ShellCheck SC2004-preferred). zsh cannot represent 2^63 in
// arithmetic at all -- documented residual. This is the single choke point for
// any litAtom entering $(( )), so it covers the genUnary literal, the bare-ident
// const, and the cross-module FieldAccess const paths.
func (g *gen) arith(a atom) string {
	if a.lit {
		if a.name == "-9223372036854775808" {
			t := g.newTemp()
			g.line("%s=-9223372036854775808", t)
			return t // BARE, not "$"+t -- $t re-lexes 2^63 and is dash-wrong.
		}
		return a.name
	}
	// Variables are referenced BARE (no leading $) for the same reason the INT_MIN
	// literal spill above is: inside $(( )) a bare name reads the variable's stored
	// value directly, which dash/busybox/bash/sh evaluate correctly even for
	// INT_MIN, whereas "$"+name string-expands the value into the expression text
	// and the shell re-lexes it -- so a stored -9223372036854775808 (magnitude
	// 2^63, one past INT_MAX) makes dash off-by-one. Every int value wisp stores
	// is canonical [+-]?[0-9]+ (no octal ambiguity, no indirection), so a bare
	// operand reads exactly that integer; it is also injection-safe (the value
	// never enters the expression text) and ShellCheck SC2004-preferred. zsh
	// cannot represent 2^63 in $(( )) at all, even bare -- documented residual.
	return a.name
}

// genExpr lowers e, emitting any needed statements, and returns an atom naming
// its value. Subexpressions evaluate strictly left to right and every
// value-producer is spilled to a fresh temp before the next __ret write (spec
// section 9.2).
func (g *gen) genExpr(e ast.Expr) atom {
	switch n := e.(type) {
	case *ast.IntLit:
		// Canonicalize the decimal literal (wisp ints are base-10 digit runs, no
		// hex/octal/separators) so a non-canonical form like 007 emits as 7. This
		// keeps int values consistent everywhere; in particular a switch subject
		// matches the canonicalized case patterns, which are emitted as their
		// folded decimal value. ParseInt only fails for the MinInt64 magnitude
		// literal (handled under unary minus); fall back to the raw text there.
		if v, err := strconv.ParseInt(n.Raw, 10, 64); err == nil {
			return litAtom(strconv.FormatInt(v, 10))
		}
		return litAtom(n.Raw)
	case *ast.FloatLit:
		// A float literal is its decimal source text. It matches the float-validity
		// invariant [+-]?[0-9]+(\.[0-9]+)? and contains no shell-active byte, so it
		// is a safe self-contained word (spec 3.1/3.6).
		return litAtom(n.Raw)
	case *ast.BoolLit:
		if n.Value {
			return litAtom("true")
		}
		return litAtom("false")
	case *ast.StringLit:
		return g.genString(n)
	case *ast.Ident:
		return g.genIdent(n)
	case *ast.UnaryExpr:
		return g.genUnary(n)
	case *ast.BinaryExpr:
		return g.genBinary(n)
	case *ast.CallExpr:
		return g.genCall(n)
	case *ast.StructLit:
		return g.genStructLit(n)
	case *ast.ArrayLit:
		return g.genArrayLit(n)
	case *ast.DictLit:
		return g.genDictLit(n)
	case *ast.TupleLit:
		return g.genTupleLit(n)
	case *ast.FieldAccess:
		// Namespaced-member funcref (Part 3): `ns.member` in value position that
		// the checker resolved to a funcref, either a builtin member or a
		// cross-module exported user function. Emit the mangled name (the SAME
		// wrapper name the bare-ident path uses for builtins, or the function's
		// own mangled shell name for a user function) and tree-shake it in.
		if fr, ok := g.info.MemberFuncRefs[n]; ok {
			if runtime.IsBuiltinWrapperID(fr.Mangled) {
				g.use(fr.Mangled)
				g.useBuiltinFuncrefExtras(fr.Mangled)
			}
			return litAtom(fr.Mangled)
		}
		// Cross-module const carrier (R4, the C-1 analog): a resolved qualified
		// const reference is a FieldAccess whose folded value the checker recorded
		// in info.FoldedValues. Inline it as a safe literal BEFORE genFieldAccess,
		// which would otherwise emit a struct field-load for an unset handle var.
		if fv, ok := g.info.FoldedValues[n]; ok {
			return g.foldedLitAtom(fv)
		}
		// Bare no-payload tagged-union construction (`Enum.Unit`): has no
		// folded value, so must be caught before genFieldAccess, which would
		// otherwise emit a struct field-load for an unset handle var.
		if bc, ok := g.info.BareEnumConstructs[n]; ok {
			return g.genBareEnumConstruct(bc)
		}
		return g.genFieldAccess(n)
	case *ast.IndexExpr:
		return g.genIndexExpr(n)
	default:
		// Every concrete ast.Expr implementor has an explicit case above.
		// ast.Expr is a sealed interface (exprNode() is unexported to
		// internal/ast), so this default cannot be reached by any value
		// constructible outside internal/ast, and cannot be synthetically
		// drift-tested from this package's own test files. It is exercised
		// only by the full regression suite confirming it stays unreached.
		panic(fmt.Sprintf("genExpr: no codegen case for %T (checker/codegen drift)", n))
	}
}

func (g *gen) genIdent(n *ast.Ident) atom {
	// Reserved constants stdout/stderr resolve to the literal fd at compile time;
	// they are not recorded in Info.Uses.
	switch n.Name {
	case "stdout":
		return litAtom("1")
	case "stderr":
		return litAtom("2")
	case "None":
		// Selected by the AST node, NOT the recorded type: a blessed-site None has
		// a concrete Optional[T] recorded in info.Types, but it still lowers to a
		// fresh tag=none handle.
		return g.genNone()
	}
	// A function reference (M4): the value is the target's mangled shell name
	// (the SAME name codegen emits for its definition -- one source of truth). The
	// name is a compiler-controlled __wisp_f_* identifier ([A-Za-z0-9_] only), so
	// it is a safe self-contained literal word with no user-influenced bytes.
	if fr, ok := g.info.FuncRefs[n]; ok {
		// For builtin funcrefs, g.use the eta-expansion wrapper (its snippet id IS
		// the mangled name) so it is tree-shaken in only when the builtin is
		// actually referenced as a value, never for a direct call.
		if runtime.IsBuiltinWrapperID(fr.Mangled) {
			g.use(fr.Mangled)
			g.useBuiltinFuncrefExtras(fr.Mangled)
		}
		return litAtom(fr.Mangled)
	}
	// Const inlining (C-1): check IsConst BEFORE the varAtom(Mangled) path.
	// Const Vars have Mangled=="" and no runtime variable; emit the folded value
	// as a safe literal atom. This covers ordinary value position AND default-arg
	// fill (defaults flow through genArgWords -> genExpr -> genIdent).
	if u := g.info.Uses[n]; u != nil && u.IsConst {
		return g.genConstLiteral(n)
	}
	v := g.info.Uses[n]
	return varAtom(v.Mangled)
}

// foldedLitAtom emits a const's folded compile-time value as a safe shell
// literal atom: int -> decimal digit string, bool -> true/false, string ->
// single-quoted via shellSingleQuote (injection-safe), float (FoldedFloat) ->
// its raw decimal word (matching a normal FloatLit). It is the single source of
// truth for inlining a folded value, shared by genConstLiteral (bare-ident
// consts, keyed via Var.FoldedValue) and the FieldAccess cross-module const
// path (keyed via info.FoldedValues). The value is re-encoded, never pasted as
// source text and never re-evaluated, so it is inert data.
func (g *gen) foldedLitAtom(fv interface{}) atom {
	switch cv := fv.(type) {
	case int64:
		return litAtom(strconv.FormatInt(cv, 10))
	case bool:
		if cv {
			return litAtom("true")
		}
		return litAtom("false")
	case string:
		return litAtom(shellSingleQuote(cv))
	case types.FoldedFloat:
		return litAtom(cv.Raw)
	}
	// Unreachable for well-typed programs; the checker guarantees a folded value.
	return litAtom("''")
}

// genConstLiteral inlines the folded value of a const identifier as a safe
// shell literal atom. Delegates to foldedLitAtom (the single source of truth
// for folded-value -> literal encoding) using the Var.FoldedValue set by the
// checker.
func (g *gen) genConstLiteral(n *ast.Ident) atom {
	return g.foldedLitAtom(g.info.Uses[n].FoldedValue)
}

// genString lowers a string literal. A pure-literal string (no interpolation)
// is a single-quoted token. An interpolated string lowers to a concatenation of
// individually quoted pieces assigned into a temp (spec sections 5.1, 9.6
// invariant 2): each literal chunk is a single-quoted literal and each ${expr}
// is the quoted expansion of its evaluated value -- inserted as data, never
// spliced as text, never re-evaluated.
func (g *gen) genString(n *ast.StringLit) atom {
	if len(n.Parts) == 0 {
		return litAtom("''")
	}
	if len(n.Parts) == 1 && n.Parts[0].IsText() {
		return litAtom(shellSingleQuote(n.Parts[0].Text))
	}
	// Evaluate every interpolation expression first (left to right), spilling each
	// to its own carrier so a later piece's evaluation cannot clobber an earlier
	// one before the concatenation is assembled.
	rendered := make([]string, len(n.Parts))
	for i, part := range n.Parts {
		if part.IsText() {
			rendered[i] = shellSingleQuote(part.Text)
			continue
		}
		// int and bool already store as text, so the evaluated value is used
		// directly as data (no conversion needed); ${} auto-stringifies (5.1).
		rendered[i] = g.word(g.genExpr(part.Expr))
	}
	t := g.newTemp()
	// Concatenate. Quoted adjacent words concatenate in the shell.
	rhs := rendered[0]
	for _, r := range rendered[1:] {
		rhs += r
	}
	g.line("%s=%s", t, rhs)
	return varAtom(t)
}

func (g *gen) genUnary(n *ast.UnaryExpr) atom {
	switch n.Op {
	case token.Minus:
		if g.resolveType(g.info.Types[n.X]) == types.Float {
			// Float negation: 0 - x via __wisp_fsub (located position from the
			// operator). The "0" operand is invariant-valid.
			x := g.genExpr(n.X)
			return g.emitFloatBinary(runtime.FSub, "__wisp_fsub", litAtom("0"), x, n.OpPos)
		}
		if il, ok := n.X.(*ast.IntLit); ok {
			// Negative INT_MIN literal: emit the negative literal directly instead
			// of $(( 0 - magnitude )), which would put the out-of-range 2^63
			// magnitude into $(( )). The returned lit atom is a valid shell word in
			// word context (correct on all shells) and is spilled to a temp by
			// arith() in arithmetic context. Only the exact INT_MIN value takes
			// this branch; all other negative literals keep the $(( 0 - mag )) form.
			if v, err := strconv.ParseInt("-"+il.Raw, 10, 64); err == nil && v == math.MinInt64 {
				return litAtom(strconv.FormatInt(v, 10))
			}
		}
		x := g.genExpr(n.X)
		t := g.newTemp()
		// 0 - x keeps the result a digit string (invariant 5).
		g.line("%s=$(( 0 - %s ))", t, g.arith(x))
		return varAtom(t)
	case token.Bang:
		x := g.genExpr(n.X)
		t := g.newTemp()
		g.line("if [ %s = true ]; then %s=false; else %s=true; fi", g.word(x), t, t)
		return varAtom(t)
	default:
		panic(fmt.Sprintf("genUnary: no codegen case for operator %s (checker/codegen drift)", n.Op))
	}
}

func (g *gen) genBinary(n *ast.BinaryExpr) atom {
	switch n.Op {
	case token.AndAnd, token.OrOr:
		return g.genShortCircuit(n)
	}

	lt := g.resolveType(g.info.Types[n.L])
	l := g.genExpr(n.L)
	r := g.genExpr(n.R)

	if lt == types.Float {
		return g.genFloatBinary(n, l, r)
	}

	switch n.Op {
	case token.Plus:
		if lt == types.String {
			// string concatenation: quoted operands, no re-expansion (section 9.4).
			t := g.newTemp()
			g.line("%s=%s%s", t, g.word(l), g.word(r))
			return varAtom(t)
		}
		return g.genArith(l, "+", r)
	case token.Minus:
		return g.genArith(l, "-", r)
	case token.Star:
		return g.genArith(l, "*", r)
	case token.Slash:
		return g.genIntDivMod(l, "/", r, n.OpPos)
	case token.Percent:
		return g.genIntDivMod(l, "%", r, n.OpPos)
	case token.Amp:
		return g.genArith(l, "&", r)
	case token.Pipe:
		return g.genArith(l, "|", r)
	case token.Caret:
		return g.genArith(l, "^", r)
	case token.Shl:
		return g.genArith(l, "<<", r)
	case token.Shr:
		return g.genArith(l, ">>", r)
	case token.Lt:
		return g.genIntCompare(l, "-lt", r)
	case token.Lte:
		return g.genIntCompare(l, "-le", r)
	case token.Gt:
		return g.genIntCompare(l, "-gt", r)
	case token.Gte:
		return g.genIntCompare(l, "-ge", r)
	case token.Eq:
		if types.ComparableOptional(lt) {
			return g.genOptionalEquality(l, r, lt, false, n.OpPos)
		}
		return g.genEquality(l, "=", r)
	case token.Neq:
		if types.ComparableOptional(lt) {
			return g.genOptionalEquality(l, r, lt, true, n.OpPos)
		}
		return g.genEquality(l, "!=", r)
	default:
		panic(fmt.Sprintf("genBinary: no codegen case for operator %s (checker/codegen drift)", n.Op))
	}
}

// genFloatBinary lowers a binary op whose operands are floats: arithmetic to the
// __wisp_f{add,sub,mul,div} helpers (which validate finiteness and abort located
// on overflow/non-finite), comparisons to __wisp_fcmp. Operand atoms are passed
// as words to the helper (values flow into awk via -v inside the helper); the
// operator position is the located <pos> for any abort.
func (g *gen) genFloatBinary(n *ast.BinaryExpr, l, r atom) atom {
	switch n.Op {
	case token.Plus:
		return g.emitFloatBinary(runtime.FAdd, "__wisp_fadd", l, r, n.OpPos)
	case token.Minus:
		return g.emitFloatBinary(runtime.FSub, "__wisp_fsub", l, r, n.OpPos)
	case token.Star:
		return g.emitFloatBinary(runtime.FMul, "__wisp_fmul", l, r, n.OpPos)
	case token.Slash:
		// __wisp_fdiv guards a numeric-zero divisor itself (located), so no
		// separate guard is emitted here.
		return g.emitFloatBinary(runtime.FDiv, "__wisp_fdiv", l, r, n.OpPos)
	case token.Lt:
		return g.emitFloatCompare("lt", l, r, n.OpPos)
	case token.Lte:
		return g.emitFloatCompare("le", l, r, n.OpPos)
	case token.Gt:
		return g.emitFloatCompare("gt", l, r, n.OpPos)
	case token.Gte:
		return g.emitFloatCompare("ge", l, r, n.OpPos)
	case token.Eq:
		return g.emitFloatCompare("eq", l, r, n.OpPos)
	case token.Neq:
		return g.emitFloatCompare("ne", l, r, n.OpPos)
	default:
		panic(fmt.Sprintf("genFloatBinary: no codegen case for operator %s (checker/codegen drift)", n.Op))
	}
}

// emitFloatBinary emits `<fn> <pos> <l> <r>` for a float arithmetic helper and
// reads its result into a fresh temp. The operand atoms are already spilled
// (genExpr), preserving left-to-right order.
func (g *gen) emitFloatBinary(id, fn string, l, r atom, pos token.Position) atom {
	g.use(id)
	g.line("%s %s %s %s", fn, g.posLiteral(pos), g.word(l), g.word(r))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// emitFloatCompare emits `__wisp_fcmp <pos> <op> <l> <r>` and reads the
// resulting true/false bool into a fresh temp. op is a compiler-fixed token
// (lt/le/gt/ge/eq/ne), single-quoted so it is a safe constant word.
func (g *gen) emitFloatCompare(op string, l, r atom, pos token.Position) atom {
	g.use(runtime.FCmp)
	g.line("__wisp_fcmp %s %s %s %s", g.posLiteral(pos), shellSingleQuote(op), g.word(l), g.word(r))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

func (g *gen) genArith(l atom, op string, r atom) atom {
	t := g.newTemp()
	g.line("%s=$(( %s %s %s ))", t, g.arith(l), op, g.arith(r))
	return varAtom(t)
}

// genIntDivMod lowers integer `/` or `%` with the div/mod-by-zero guard. The
// guard runs FIRST; in errMode the mode-aware __wisp_fail RETURNS at depth>0
// (it does not exit), so the `$(( ))` MUST be skipped when a fault is pending --
// otherwise `$(( l / 0 ))` is a POSIX-fatal arithmetic error that hard-aborts
// the whole shell, bypassing the catch. We emit the zero-guard, then run the
// arithmetic only when not pending (`[ -n pending ] || t=$(( ))`), and open a
// post-spill guard so the rest of the statement is skipped on the fault. Outside
// errMode this is the plain M4 guard + arithmetic (pending is never set).
func (g *gen) genIntDivMod(l atom, op string, r atom, pos token.Position) atom {
	g.guardDivZero(r, pos)
	// `/` can also overflow at INT_MIN / -1 (unrepresentable quotient, fatal $(( ))
	// on some shells); guard it before the arithmetic.
	if op == "/" {
		g.guardDivOverflow(l, r, pos)
	}
	t := g.newTemp()
	// `%` at INT_MIN % -1 has the representable result 0, but x86 idiv still traps
	// (SIGFPE) computing the overflowing quotient alongside the remainder. When the
	// divisor may be -1 and the dividend may be INT_MIN, substitute 0 instead of
	// evaluating the arithmetic (see modMayOverflow / modGuardExpr).
	if op == "%" && g.modMayOverflow(l, r) {
		expr := g.modGuardExpr(t, l, r)
		if g.errMode {
			g.line("[ -n \"$__wisp_err_pending\" ] || %s", expr)
			g.guardAfterSpill()
			return varAtom(t)
		}
		g.line("%s", expr)
		return varAtom(t)
	}
	if g.errMode {
		g.line("[ -n \"$__wisp_err_pending\" ] || %s=$(( %s %s %s ))", t, g.arith(l), op, g.arith(r))
		g.guardAfterSpill()
		return varAtom(t)
	}
	g.line("%s=$(( %s %s %s ))", t, g.arith(l), op, g.arith(r))
	return varAtom(t)
}

// modMayOverflow reports whether `l % r` could be the trapping INT_MIN % -1 case.
// It is false when the divisor is a literal other than -1 (can never be -1) or the
// dividend is a literal other than INT_MIN (can never be INT_MIN); those keep the
// plain single-line arithmetic and leave existing snapshots unchanged. Otherwise
// the guard is emitted.
func (g *gen) modMayOverflow(l, r atom) bool {
	if r.lit && r.name != "-1" {
		return false
	}
	if l.lit && l.name != "-9223372036854775808" {
		return false
	}
	return true
}

// modGuardExpr emits any operand spills and returns a one-line shell `if` that
// assigns t the representable result 0 for INT_MIN % -1 (detected via IMin, which
// avoids the 19-digit literal for zsh) and otherwise evaluates t=$(( l % r )) with
// bare operands. Both operands are materialized into variables first so each can be
// referenced bare inside $(( )) and as "$name" in the [ ] tests. zsh cannot do
// 2^63 arithmetic at all, even here -- the same documented INT_MIN residual.
func (g *gen) modGuardExpr(t string, l, r atom) string {
	lv := g.toVar(l)
	rv := g.toVar(r)
	g.use(runtime.IMin)
	return fmt.Sprintf("if [ \"$%s\" -eq -1 ] && { __wisp_imin; [ \"$%s\" -eq \"$__ret\" ]; }; then %s=0; else %s=$(( %s %% %s )); fi",
		rv, lv, t, t, lv, rv)
}

// toVar returns the name of a shell variable holding a's value, spilling a literal
// to a temp via a plain (non-arith) word assignment. Unlike arith(), which returns
// bare digits for a non-INT_MIN literal, toVar guarantees a variable so the value
// can be used both bare inside $(( )) and as "$name" in a [ ] test.
func (g *gen) toVar(a atom) string {
	if !a.lit {
		return a.name
	}
	t := g.newTemp()
	g.line("%s=%s", t, a.name)
	return t
}

// guardDivOverflow emits the INT_MIN / -1 overflow guard via __wisp_idiv_ovf,
// passing the operator position as the located <pos>. It runs after the zero
// guard and before the `$(( )) `. The helper is a no-op unless the divisor is -1
// and the dividend is INT_MIN, where it aborts located ("division overflow").
func (g *gen) guardDivOverflow(l, r atom, pos token.Position) {
	g.use(runtime.IDivOvf)
	g.line("__wisp_idiv_ovf %s %s %s", g.posLiteral(pos), g.word(l), g.word(r))
}

// guardDivZero emits the div/mod-by-zero guard. pos is the operator position,
// re-encoded as a safe shell literal and passed as the located <pos> argument to
// __wisp_fail. The "division by zero" label is preserved verbatim from M1 for
// both / and % (the shared guard); only the located position is added (M2).
func (g *gen) guardDivZero(r atom, pos token.Position) {
	g.use(runtime.Fail)
	g.line("if [ %s -eq 0 ]; then __wisp_fail %s \"division by zero\"; fi", g.divGuardWord(r), g.posLiteral(pos))
}

// posLiteral re-encodes a source position (file:line:col) as a safe POSIX
// single-quoted shell literal (spec section 4 / M1 section 9.6 invariant 1).
// The source file path may contain shell-active characters; it is never pasted
// verbatim between quotes.
func (g *gen) posLiteral(pos token.Position) string {
	return shellSingleQuote(pos.String())
}

// divGuardWord renders r for the div-by-zero guard `[ <r> -eq 0 ]`. A variable
// is double-quoted; a literal is the digits.
func (g *gen) divGuardWord(r atom) string {
	if r.lit {
		return r.name
	}
	return "\"$" + r.name + "\""
}

// genIntCompare emits an int comparison capturing true/false (section 9.4).
func (g *gen) genIntCompare(l atom, op string, r atom) atom {
	t := g.newTemp()
	g.line("if [ %s %s %s ]; then %s=true; else %s=false; fi", g.cmpWord(l), op, g.cmpWord(r), t, t)
	return varAtom(t)
}

// cmpWord renders an int operand for a `[ ]` numeric test: digits for a literal,
// a quoted expansion for a variable (every int value is [+-]?[0-9]+, invariant 5).
func (g *gen) cmpWord(a atom) string {
	if a.lit {
		return a.name
	}
	return "\"$" + a.name + "\""
}

// emitScalarEq tests equality of two already-spilled scalar operands (variable
// NAMES, not atoms) and returns a fresh temp holding "true"/"false". For float
// operands it routes through __wisp_fcmp so numerically-equal floats are equal
// (1.0==1.00, and -0.0==0.0) -- a text `=` compare would split them. For
// int/bool/string it uses the text `=` test. pos is the located position for the
// fcmp helper (unused on the text path).
func (g *gen) emitScalarEq(aName, bName string, isFloat bool, pos token.Position) string {
	r := g.newTemp()
	if isFloat {
		g.use(runtime.FCmp)
		g.line("__wisp_fcmp %s %s \"$%s\" \"$%s\"", g.posLiteral(pos), shellSingleQuote("eq"), aName, bName)
		g.line("%s=\"$__ret\"", r)
	} else {
		g.line("if [ \"$%s\" = \"$%s\" ]; then %s=true; else %s=false; fi", aName, bName, r, r)
	}
	return r
}

// comparesAsFloat reports whether a value of type t must compare by NUMERIC
// identity (__wisp_fcmp) rather than shell byte-text: a plain float, OR a
// float-backed value enum (whose runtime value IS its backing float text).
// This is the single source of truth reused by the membership builtins (this
// task), the assert family, Optional equality, and the float switch.
func (g *gen) comparesAsFloat(t types.Type) bool {
	rt := g.resolveType(t)
	if rt == types.Float {
		return true
	}
	if ei, ok := g.info.Enums[string(rt)]; ok && ei.Kind == types.EnumValue && ei.Backing == types.Float {
		return true
	}
	return false
}

// genEquality emits == / != for int, bool, or string operands. All three lower
// to a string `=`/`!=` test on the shared text representation; the type system
// guarantees both operands share a type so this is sound (spec section 6).
func (g *gen) genEquality(l atom, op string, r atom) atom {
	t := g.newTemp()
	g.line("if [ %s %s %s ]; then %s=true; else %s=false; fi", g.word(l), op, g.word(r), t, t)
	return varAtom(t)
}

// genShortCircuit lowers && / || with short-circuit evaluation (section 9.4):
// the right operand's value-producing code runs only when needed. The result is
// a bool temp.
func (g *gen) genShortCircuit(n *ast.BinaryExpr) atom {
	t := g.newTemp()
	l := g.genExpr(n.L)
	if n.Op == token.AndAnd {
		// result = if l==true then (eval r) else false
		g.line("if [ %s = true ]; then", g.word(l))
		g.indent++
		r := g.genExpr(n.R)
		g.line("%s=%s", t, g.word(r))
		g.indent--
		g.line("else")
		g.indent++
		g.line("%s=false", t)
		g.indent--
		g.line("fi")
	} else {
		// result = if l==true then true else (eval r)
		g.line("if [ %s = true ]; then", g.word(l))
		g.indent++
		g.line("%s=true", t)
		g.indent--
		g.line("else")
		g.indent++
		r := g.genExpr(n.R)
		g.line("%s=%s", t, g.word(r))
		g.indent--
		g.line("fi")
	}
	return varAtom(t)
}

// genCond evaluates a predicate and returns the name of a bool temp holding its
// true/false value, ready to be tested with `[ "$name" = true ]` (section 9.4).
func (g *gen) genCond(e ast.Expr) string {
	// Any skip-guard opened while evaluating the predicate (after a call spill)
	// closes within this scope, so the predicate's value is captured and a
	// re-evaluated loop condition does not leave a guard open across iterations
	// (M5). If the predicate faulted, the captured value is stale, but every
	// statement gated on it is itself guarded and skips, and a loop's top-of-body
	// pending-break exits the loop -- so correctness holds (fail-at-first-fault).
	base := g.guardDepth
	a := g.genExpr(e)
	c := g.newCond()
	g.line("%s=%s", c, g.word(a))
	g.closeGuardsTo(base)
	return c
}

func (g *gen) genCall(n *ast.CallExpr) atom {
	ci := g.info.Calls[n]
	switch ci.Kind {
	case types.CallUser:
		return g.genUserCall(n, ci)
	case types.CallBuiltin:
		return g.genBuiltinCall(n, ci)
	case types.CallIndirect:
		return g.genIndirectCall(n, ci)
	case types.CallEnumConstruct:
		return g.genEnumConstruct(ci)
	default:
		panic(fmt.Sprintf("genCall: no codegen case for CallKind %d (checker/codegen drift)", ci.Kind))
	}
}

// genIndirectCall lowers a call through a function reference (M4). The callee
// expression is evaluated to a funcref value (a __wisp_f_* name) and spilled to
// a temp; the call is `"$ftmp" "$a" "$b"`, reusing the M1 spill/__ret discipline.
// __ret is read into a fresh temp only for a non-void result (a void callee --
// e.g. used by each -- leaves __ret untouched, so it is not read).
func (g *gen) genIndirectCall(n *ast.CallExpr, ci *types.CallInfo) atom {
	callee := g.genExpr(n.Callee)
	ftmp := g.spillToTemp(callee)
	words := g.genArgWords(ci.Args)
	g.line("\"$%s\"%s", ftmp, joinWords(words))
	if ci.Result == types.Void {
		g.guardAfterSpill()
		return litAtom("''")
	}
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

func (g *gen) genUserCall(n *ast.CallExpr, ci *types.CallInfo) atom {
	// Evaluate args left to right into stable words.
	words := g.genArgWords(ci.Args)
	g.line("%s%s", g.instantiatedCallName(ci), joinWords(words))
	if ci.Result == types.Void {
		// A faulted callee returns with pending set; guard so the rest of this
		// statement's evaluation is skipped (fail-at-first-fault, mid-statement).
		g.guardAfterSpill()
		return litAtom("''")
	}
	// Read __ret immediately into a fresh temp before any other call clobbers it.
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genArgWords evaluates each argument expression to an atom and renders it as a
// command-line word, preserving left-to-right order. Each non-trivial argument
// is spilled to its own temp by genExpr, so later arguments cannot clobber
// earlier ones.
func (g *gen) genArgWords(args []ast.Expr) []string {
	words := make([]string, len(args))
	for i, a := range args {
		words[i] = g.word(g.genExpr(a))
	}
	return words
}

func joinWords(words []string) string {
	out := ""
	for _, w := range words {
		out += " " + w
	}
	return out
}

func (g *gen) genBuiltinCall(n *ast.CallExpr, ci *types.CallInfo) atom {
	switch ci.Builtin {
	case "print":
		// print(msg, to): to is a reserved-constant ident resolving to fd 1/2.
		msg := g.genExpr(ci.Args[0])
		fd := g.genExpr(ci.Args[1]) // litAtom "1" or "2"
		g.use(runtime.Print)
		g.line("__wisp_print %s %s", g.word(msg), fd.name)
		return litAtom("''")
	case "to_string":
		// to_string(float) canonicalizes via awk (%.17g); int/bool/string store as
		// text and are identity. Dispatch by the static argument type.
		if g.info.Types[ci.Args[0]] == types.Float {
			return g.genLocatedHelperCall(runtime.FStr, "__wisp_fstr", n, ci.Args)
		}
		return g.genHelperCall(runtime.String, "__wisp_string", n, ci.Args)
	case "to_float":
		// to_float(int) and to_float(string) per spec 3.4. The string path validates
		// via a case-glob before use; both validate finiteness. Located <pos>.
		if g.info.Types[ci.Args[0]] == types.Int {
			return g.genLocatedHelperCall(runtime.FFloatI, "__wisp_ffloat_i", n, ci.Args)
		}
		return g.genLocatedHelperCall(runtime.FFloatS, "__wisp_ffloat_s", n, ci.Args)
	case "length":
		// length(T[]) returns the array element count (the array's _len var);
		// length(string) returns byte count. Dispatch by the static arg type.
		if types.IsArray(g.info.Types[ci.Args[0]]) {
			return g.genArrayLength(ci.Args[0])
		}
		return g.genHelperCall(runtime.Length, "__wisp_length", n, ci.Args)
	case "push":
		return g.genPush(ci.Args)
	case "has":
		return g.genHas(n, ci.Args)
	case "keys":
		return g.genKeys(ci.Args)
	case "map":
		if types.IsOptional(g.info.Types[ci.Args[0]]) {
			return g.genMapTagged(ci.Args, "some")
		}
		if types.IsResult(g.info.Types[ci.Args[0]]) {
			return g.genMapTagged(ci.Args, "ok")
		}
		return g.genMap(ci.Args)
	case "filter":
		if types.IsOptional(g.info.Types[ci.Args[0]]) {
			return g.genFilterOptional(ci.Args)
		}
		return g.genFilter(ci.Args)
	case "and_then":
		if types.IsOptional(g.info.Types[ci.Args[0]]) {
			return g.genAndThenTagged(ci.Args, "some")
		}
		return g.genAndThenTagged(ci.Args, "ok")
	case "or_else":
		if types.IsOptional(g.info.Types[ci.Args[0]]) {
			return g.genOrElseOptional(ci.Args)
		}
		return g.genOrElseResult(ci.Args)
	case "map_err":
		return g.genMapErrResult(ci.Args)
	case "each":
		return g.genEach(ci.Args)
	case "zip":
		return g.genZip(n, ci.Args)
	case "parse_args":
		return g.genParseArgs(ci.Args)
	case "error":
		return g.genErrorLit(ci.Args)
	case "error_with":
		return g.genErrorWithLit(ci.Args)
	case "wrap":
		return g.genWrap(ci.Args)
	case "cause":
		return g.genCause(ci.Args)
	case "Some":
		return g.genSome(ci.Args)
	case "is_some":
		return g.genIsSome(ci.Args, "some")
	case "is_none":
		return g.genIsSome(ci.Args, "none")
	case "Ok":
		return g.genOk(ci.Args)
	case "Err":
		return g.genErr(ci.Args)
	case "is_ok":
		return g.genIsOk(ci.Args, "ok")
	case "is_err":
		return g.genIsOk(ci.Args, "err")
	case "unwrap_err":
		return g.genUnwrapErr(n, ci.Args)
	case "unwrap":
		// Overloaded: dispatch on the operand's static type (Optional vs Result).
		if types.IsResult(g.info.Types[ci.Args[0]]) {
			return g.genUnwrapResult(n, ci.Args)
		}
		return g.genUnwrap(n, ci.Args)
	case "unwrap_or":
		if types.IsResult(g.info.Types[ci.Args[0]]) {
			return g.genUnwrapOrResult(ci.Args)
		}
		return g.genUnwrapOr(ci.Args)
	case "lower":
		return g.genHelperCall(runtime.Lower, "__wisp_lower", n, ci.Args)
	case "upper":
		return g.genHelperCall(runtime.Upper, "__wisp_upper", n, ci.Args)
	case "trim":
		return g.genHelperCall(runtime.Trim, "__wisp_trim", n, ci.Args)
	case "replace":
		// replace() can abort (empty search); pass the located call position.
		return g.genLocatedHelperCall(runtime.Replace, "__wisp_replace", n, ci.Args)
	case "matches":
		return g.genLocatedHelperCall(runtime.Matches, "__wisp_matches", n, ci.Args)
	case "regex_replace":
		return g.genLocatedHelperCall(runtime.RegexReplace, "__wisp_regex_replace", n, ci.Args)
	case "regex_find":
		return g.genLocatedTokenToOptional(runtime.RegexFind, "__wisp_regex_find", n, ci.Args)
	case "regex_find_all":
		return g.genRegexFindAll(n, ci.Args)
	case "to_int":
		// to_int(enum) is identity at the value level (an enum value already IS its
		// int; the variant access folded to an int literal). Emit the operand
		// directly with no parse/range helper.
		if _, isEnum := g.info.Enums[string(g.info.Types[ci.Args[0]])]; isEnum {
			return g.genExpr(ci.Args[0])
		}
		// to_int(float) truncates toward zero awk-side then reuses __wisp_int's range
		// check; to_int(string) parses. Both can abort; pass the located position.
		if g.info.Types[ci.Args[0]] == types.Float {
			return g.genLocatedHelperCall(runtime.FIntT, "__wisp_fint", n, ci.Args)
		}
		return g.genLocatedHelperCall(runtime.Int, "__wisp_int", n, ci.Args)
	case "to_bool":
		// Dispatch by the static argument type (section 10.1): int->bool,
		// float->bool, and string->bool obey different rules. The float path tests
		// numeric zero via awk; only the string path can abort.
		argType := g.info.Types[ci.Args[0]]
		if argType == types.Int {
			return g.genHelperCall(runtime.BoolInt, "__wisp_bool_int", n, ci.Args)
		}
		if argType == types.Float {
			return g.genLocatedHelperCall(runtime.FBool, "__wisp_fbool", n, ci.Args)
		}
		return g.genLocatedHelperCall(runtime.BoolStr, "__wisp_bool_str", n, ci.Args)
	case "split":
		return g.genSplit(n, ci.Args)
	case "join":
		return g.genJoin(ci.Args)
	case "contains":
		// Overloaded like length: dispatch the substring/membership variant on the
		// static arg-1 type.
		if types.IsArray(g.info.Types[ci.Args[0]]) {
			return g.genArrayContains(ci.Args)
		}
		return g.genHelperCall(runtime.Contains, "__wisp_contains", n, ci.Args)
	case "starts_with":
		return g.genHelperCall(runtime.StartsWith, "__wisp_starts_with", n, ci.Args)
	case "ends_with":
		return g.genHelperCall(runtime.EndsWith, "__wisp_ends_with", n, ci.Args)
	case "index_of":
		if types.IsArray(g.info.Types[ci.Args[0]]) {
			return g.genIndexOfElem(ci.Args)
		}
		return g.genIntSentinelToOptional(runtime.IndexOf, "__wisp_index_of", n, ci.Args)
	case "repeat":
		// repeat() can abort (n<0); pass the located call position.
		return g.genLocatedHelperCall(runtime.Repeat, "__wisp_repeat", n, ci.Args)
	case "abs":
		return g.genAbs(n, ci.Args)
	case "min":
		return g.genMinMax(ci.Args, true)
	case "max":
		return g.genMinMax(ci.Args, false)
	case "reverse":
		return g.genReverse(ci.Args)
	case "reduce":
		return g.genReduce(ci.Args)
	case "env":
		// env can abort (unset); pass the located call position.
		return g.genLocatedHelperCall(runtime.Env, "__wisp_env", n, ci.Args)
	case "has_env":
		return g.genHelperCall(runtime.HasEnv, "__wisp_has_env", n, ci.Args)
	case "read_file":
		return g.genLocatedHelperCall(runtime.ReadFile, "__wisp_read_file", n, ci.Args)
	case "write_file":
		return g.genVoidLocatedHelperCall(runtime.WriteFile, "__wisp_write_file", n, ci.Args)
	case "append_file":
		return g.genVoidLocatedHelperCall(runtime.AppendFile, "__wisp_append_file", n, ci.Args)
	case "set_env":
		return g.genVoidLocatedHelperCall(runtime.SetEnv, "__wisp_set_env", n, ci.Args)
	case "unset_env":
		return g.genVoidLocatedHelperCall(runtime.UnsetEnv, "__wisp_unset_env", n, ci.Args)
	case "set_stdin":
		return g.genVoidLocatedHelperCall(runtime.SetStdin, "__wisp_set_stdin", n, ci.Args)
	case "run":
		return g.genRun(n, ci.Args)
	case "run_env":
		return g.genRunEnv(n, ci.Args)
	case "exit":
		return g.genExit(ci.Args)
	case "on_exit":
		return g.genOnExit(ci.Args)
	case "on_signal":
		return g.genOnSignal(ci.Args)
	case "pid_alive":
		return g.genHelperCall(runtime.PidAlive, "__wisp_pid_alive", n, ci.Args)
	case "file_exists":
		return g.genHelperCall(runtime.FileExists, "__wisp_file_exists", n, ci.Args)
	case "is_dir":
		return g.genHelperCall(runtime.IsDir, "__wisp_is_dir", n, ci.Args)
	case "is_file":
		return g.genHelperCall(runtime.IsFile, "__wisp_is_file", n, ci.Args)
	case "is_symlink":
		return g.genHelperCall(runtime.IsSymlink, "__wisp_is_symlink", n, ci.Args)
	case "file_size":
		return g.genLocatedHelperCall(runtime.FileSize, "__wisp_file_size", n, ci.Args)
	case "chmod":
		return g.genVoidLocatedHelperCall(runtime.Chmod, "__wisp_chmod", n, ci.Args)
	case "symlink":
		return g.genVoidLocatedHelperCall(runtime.Symlink, "__wisp_symlink", n, ci.Args)
	case "symlink_force":
		return g.genVoidLocatedHelperCall(runtime.SymlinkForce, "__wisp_symlink_force", n, ci.Args)
	case "make_fifo":
		return g.genVoidLocatedHelperCall(runtime.MakeFifo, "__wisp_make_fifo", n, ci.Args)
	case "exec_command":
		return g.genVoidLocatedHelperCall(runtime.ExecCommand, "__wisp_exec_command", n, ci.Args)
	case "read_link":
		return g.genStrSentinelToOptional(runtime.ReadLink, "__wisp_read_link", n, ci.Args)
	case "temp_file":
		return g.genLocatedHelperCall(runtime.TempFile, "__wisp_temp_file", n, ci.Args)
	case "temp_dir":
		return g.genLocatedHelperCall(runtime.TempDir, "__wisp_temp_dir", n, ci.Args)
	case "cwd":
		return g.genHelperCall(runtime.Cwd, "__wisp_cwd", n, ci.Args)
	case "program_path":
		// program_path() reads the top-level $0 capture global via the
		// __wisp_program_path helper. Registering the Arg0 sentinel in `used`
		// makes codegen emit the `__wisp_arg0="$0"` capture once before main
		// (tree-shaken otherwise); Arg0 is a sentinel, not a registry dependency,
		// so it must be g.use'd explicitly alongside the helper.
		g.use(runtime.Arg0)
		return g.genHelperCall(runtime.ProgramPath, "__wisp_program_path", n, ci.Args)
	case "dir_name":
		return g.genHelperCall(runtime.DirName, "__wisp_dir_name", n, ci.Args)
	case "base_name":
		return g.genHelperCall(runtime.BaseName, "__wisp_base_name", n, ci.Args)
	case "env_or":
		return g.genHelperCall(runtime.EnvOr, "__wisp_env_or", n, ci.Args)
	case "make_dir":
		return g.genVoidLocatedHelperCall(runtime.MakeDir, "__wisp_make_dir", n, ci.Args)
	case "remove_file":
		return g.genVoidLocatedHelperCall(runtime.RemoveFile, "__wisp_remove_file", n, ci.Args)
	case "remove_dir":
		return g.genVoidLocatedHelperCall(runtime.RemoveDir, "__wisp_remove_dir", n, ci.Args)
	case "rename":
		return g.genVoidLocatedHelperCall(runtime.Rename, "__wisp_rename", n, ci.Args)
	case "which":
		return g.genStrSentinelToOptional(runtime.Which, "__wisp_which", n, ci.Args)
	case "list_dir":
		return g.genListDir(n, ci.Args)
	case "glob":
		return g.genGlob(n, ci.Args)
	case "run_status":
		return g.genRunStatus(n, ci.Args)
	case "run_env_status":
		return g.genRunEnvStatus(n, ci.Args)
	case "run_env_full":
		return g.genRunEnvFull(n, ci.Args)
	case "read_line":
		return g.genReadLine()
	case "read_secret":
		return g.genReadSecret(n, ci.Args)
	case "read_stdin":
		return g.genHelperCall(runtime.ReadStdin, "__wisp_read_stdin", n, ci.Args)
	case "change_dir":
		return g.genLocatedHelperCall(runtime.ChangeDir, "__wisp_change_dir", n, ci.Args)
	case "run_full":
		return g.genRunFull(n, ci.Args)
	case "run_input":
		return g.genRunInput(n, ci.Args)
	case "run_input_full":
		return g.genRunInputFull(n, ci.Args)
	case "pipe":
		return g.genPipe(n, ci.Args)
	case "spawn":
		return g.genSpawn(n, ci.Args)
	case "wait":
		return g.genWait(n, ci.Args)
	case "is_done":
		return g.genIsDone(n, ci.Args)
	case "signal":
		return g.genSignal(n, ci.Args)
	case "wait_any":
		return g.genWaitAny(n, ci.Args)
	case "sort":
		return g.genSort(n, ci.Args)
	case "sort_by":
		return g.genSortBy(ci.Args)
	case "find":
		return g.genFind(ci.Args)
	case "any":
		return g.genAnyAll(ci.Args, true)
	case "all":
		return g.genAnyAll(ci.Args, false)
	case "slice":
		return g.genSlice(n, ci.Args)
	case "concat":
		return g.genConcat(ci.Args)
	case "sum":
		return g.genSum(n, ci, ci.Args)
	case "range":
		return g.genRange(ci.Args)
	case "first":
		return g.genFirstLast(n, ci.Args, true)
	case "last":
		return g.genFirstLast(n, ci.Args, false)
	case "count_where":
		return g.genCountWhere(ci.Args)
	case "flatten":
		return g.genFlatten(ci.Args)
	case "unique":
		return g.genUnique(ci.Args)
	case "take":
		return g.genTakeDrop(ci.Args, true)
	case "drop":
		return g.genTakeDrop(ci.Args, false)
	case "pop":
		return g.genPop(n, ci.Args)
	case "remove_at":
		return g.genRemoveAt(n, ci.Args)
	case "insert_at":
		return g.genInsertAt(n, ci.Args)
	case "size":
		return g.genSize(ci.Args)
	case "clear":
		return g.genClear(ci.Args)
	case "values":
		return g.genValues(ci.Args)
	case "get_or":
		return g.genGetOr(ci.Args)
	case "get":
		return g.genGet(ci.Args)
	case "remove":
		return g.genRemove(ci.Args)
	case "merge":
		return g.genMerge(ci.Args)
	case "trim_start":
		return g.genHelperCall(runtime.TrimStart, "__wisp_trim_start", n, ci.Args)
	case "trim_end":
		return g.genHelperCall(runtime.TrimEnd, "__wisp_trim_end", n, ci.Args)
	case "trim_prefix":
		return g.genHelperCall(runtime.TrimPrefix, "__wisp_trim_prefix", n, ci.Args)
	case "trim_suffix":
		return g.genHelperCall(runtime.TrimSuffix, "__wisp_trim_suffix", n, ci.Args)
	case "last_index_of":
		return g.genIntSentinelToOptional(runtime.LastIndexOf, "__wisp_last_index_of", n, ci.Args)
	case "is_empty":
		return g.genHelperCall(runtime.IsEmpty, "__wisp_is_empty", n, ci.Args)
	case "substring":
		return g.genLocatedHelperCall(runtime.Substring, "__wisp_substring", n, ci.Args)
	case "char_at":
		return g.genLocatedHelperCall(runtime.CharAt, "__wisp_char_at", n, ci.Args)
	case "count":
		return g.genLocatedHelperCall(runtime.Count, "__wisp_count", n, ci.Args)
	case "replace_first":
		return g.genLocatedHelperCall(runtime.ReplaceFirst, "__wisp_replace_first", n, ci.Args)
	case "pad_start":
		return g.genLocatedHelperCall(runtime.PadStart, "__wisp_pad_start", n, ci.Args)
	case "pad_end":
		return g.genLocatedHelperCall(runtime.PadEnd, "__wisp_pad_end", n, ci.Args)
	case "lines":
		return g.genLines(ci.Args)
	case "reverse_string":
		return g.genHelperCall(runtime.ReverseString, "__wisp_reverse_string", n, ci.Args)
	case "ord":
		return g.genLocatedHelperCall(runtime.Ord, "__wisp_ord", n, ci.Args)
	case "chr":
		return g.genLocatedHelperCall(runtime.Chr, "__wisp_chr", n, ci.Args)
	case "sqrt":
		return g.genLocatedHelperCall(runtime.Sqrt, "__wisp_sqrt", n, ci.Args)
	case "format_float":
		return g.genLocatedHelperCall(runtime.FormatFloat, "__wisp_format_float", n, ci.Args)
	case "pow":
		return g.genLocatedHelperCall(runtime.Pow, "__wisp_pow", n, ci.Args)
	case "exp":
		return g.genLocatedHelperCall(runtime.Exp, "__wisp_exp", n, ci.Args)
	case "ln":
		return g.genLocatedHelperCall(runtime.Ln, "__wisp_ln", n, ci.Args)
	case "log10":
		return g.genLocatedHelperCall(runtime.Log10, "__wisp_log10", n, ci.Args)
	case "log2":
		return g.genLocatedHelperCall(runtime.Log2, "__wisp_log2", n, ci.Args)
	case "pi":
		return g.genHelperCall(runtime.Pi, "__wisp_pi", n, ci.Args)
	case "int_max":
		return g.genHelperCall(runtime.IntMax, "__wisp_int_max", n, ci.Args)
	case "int_min":
		return g.genHelperCall(runtime.IntMin, "__wisp_int_min", n, ci.Args)
	case "floor":
		return g.genLocatedHelperCall(runtime.Floor, "__wisp_floor", n, ci.Args)
	case "ceil":
		return g.genLocatedHelperCall(runtime.Ceil, "__wisp_ceil", n, ci.Args)
	case "round":
		return g.genLocatedHelperCall(runtime.Round, "__wisp_round", n, ci.Args)
	case "trunc":
		// trunc(x) is int(float): toward-zero truncation with the int-range check.
		return g.genLocatedHelperCall(runtime.FIntT, "__wisp_fint", n, ci.Args)
	case "gcd":
		return g.genLocatedHelperCall(runtime.Gcd, "__wisp_gcd", n, ci.Args)
	case "lcm":
		return g.genLocatedHelperCall(runtime.Lcm, "__wisp_lcm", n, ci.Args)
	case "int_or":
		return g.genHelperCall(runtime.IntOr, "__wisp_int_or", n, ci.Args)
	case "float_or":
		return g.genHelperCall(runtime.FloatOr, "__wisp_float_or", n, ci.Args)
	case "clamp":
		return g.genClamp(n, ci.Args)
	case "sign":
		return g.genSign(n, ci.Args)
	case "debug":
		return g.genDebug(ci.Args[0])
	case "assert":
		return g.genAssert(n, ci.Args)
	case "assert_eq":
		return g.genAssertEqNe(n, ci.Args, false)
	case "assert_ne":
		return g.genAssertEqNe(n, ci.Args, true)
	case "assert_some":
		return g.genAssertOptional(n, ci.Args, "some")
	case "assert_none":
		return g.genAssertOptional(n, ci.Args, "none")
	case "assert_ok":
		return g.genAssertResult(n, ci.Args, "ok")
	case "assert_err":
		return g.genAssertResult(n, ci.Args, "err")
	case "assert_contains":
		return g.genAssertContains(n, ci.Args)
	case "skip":
		return g.genSkip(n, ci.Args)
	case "test_tmpdir":
		return g.genTestTmpdir()
	case "now":
		return g.genHelperCall(runtime.Now, "__wisp_now", n, ci.Args)
	case "sleep":
		return g.genVoidLocatedHelperCall(runtime.Sleep, "__wisp_sleep", n, ci.Args)
	case "random":
		return g.genLocatedHelperCall(runtime.Random, "__wisp_random", n, ci.Args)
	case "json_encode":
		return g.genJSONEncode(ci.Args)
	case "json_decode":
		return g.genJSONDecode(n, ci, ci.Args)
	case "json_null":
		return g.genJSONNull()
	case "json_from_int", "json_from_bool", "json_from_float":
		return g.genJSONFromIdentity(ci.Args)
	case "json_from_string":
		return g.genJSONFromString(ci.Args)
	case "json_array":
		return g.genJSONArray(ci.Args)
	case "json_object":
		return g.genJSONObject(ci.Args)
	case "json_type_of":
		return g.genJSONTypeOf(ci.Args)
	case "json_get":
		return g.genJSONGetAt(runtime.JSONGet, "__wisp_json_get", n, ci.Args)
	case "json_at":
		return g.genJSONGetAt(runtime.JSONAt, "__wisp_json_at", n, ci.Args)
	case "json_as_string":
		return g.genJSONScalarAccessor(runtime.JSONAsString, "__wisp_json_as_string", n, ci.Args)
	case "json_as_int":
		return g.genJSONScalarAccessor(runtime.JSONAsInt, "__wisp_json_as_int", n, ci.Args)
	case "json_as_float":
		return g.genJSONScalarAccessor(runtime.JSONAsFloat, "__wisp_json_as_float", n, ci.Args)
	case "json_as_bool":
		return g.genJSONScalarAccessor(runtime.JSONAsBool, "__wisp_json_as_bool", n, ci.Args)
	default:
		panic(fmt.Sprintf("genBuiltinCall: no codegen case for builtin %q (checker/codegen drift)", ci.Builtin))
	}
}

// genVoidLocatedHelperCall lowers a fallible VOID builtin (write_file /
// append_file): it forwards the located call-site position as the leading <pos>
// argument and does NOT read __ret (the helper produces no value). A fault sets
// pending; the post-spill guard skips the rest of the statement (M5).
func (g *gen) genVoidLocatedHelperCall(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	pos := g.posLiteral(n.Pos())
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s %s%s", fn, pos, joinWords(words))
	g.guardAfterSpill()
	return litAtom("''")
}

// genRun lowers run(argv) -> string. The argv array handle id is spilled to a
// stable temp and passed to the __wisp_run helper FUNCTION as `__wisp_run <pos>
// <id>`; the helper rebuilds argv into its own positional parameters and
// executes "$@", so no command string is ever assembled (injection-safe) and the
// caller's positionals are untouched. The helper writes captured stdout to __ret;
// an empty argv or a nonzero exit aborts located.
func (g *gen) genRun(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.Run)
	g.line("__wisp_run %s \"$%s\"", g.posLiteral(n.Pos()), id)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genRunEnv lowers run_env(argv, env) -> string. It allocs a NEW array handle,
// spills the argv array handle id and the env dict handle id, and calls the
// shared __wisp_run_env_argv builder, which validates each env NAME, builds the
// `env NAME=VALUE... <argv>` prefix into the new handle, and aborts located on an
// empty original argv or an invalid env name. After guarding, it hands the new
// handle to the EXISTING __wisp_run path (stdout capture, abort on nonzero) --
// the run_env family reuses the plain run machinery unchanged.
func (g *gen) genRunEnv(n *ast.CallExpr, args []ast.Expr) atom {
	argvID := g.spillToTemp(g.genExpr(args[0]))
	envID := g.spillToTemp(g.genExpr(args[1]))
	g.use(runtime.RunEnv)
	newID := g.allocHandle()
	pos := g.posLiteral(n.Pos())
	g.line("__wisp_run_env_argv %s \"$%s\" \"$%s\" \"$%s\"", pos, newID, argvID, envID)
	g.guardAfterSpill()
	g.use(runtime.Run)
	g.line("__wisp_run %s \"$%s\"", pos, newID)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genRunStatus lowers run_status(argv) -> int. Same argv-handle spill + helper
// call shape as genRun, but the helper runs the command BARE (child stdout/stderr
// pass through) and returns the child's exit status in __ret; only an empty argv
// aborts located.
func (g *gen) genRunStatus(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.RunStatus)
	g.line("__wisp_run_status %s \"$%s\"", g.posLiteral(n.Pos()), id)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genReadLine lowers read_line() -> Optional[string]. The helper writes the line
// content to __ret and sets __wisp_rl_eof=1 on EOF (empty on success). Builds the
// Optional handle directly; total (no guard).
func (g *gen) genReadLine() atom {
	g.use(runtime.ReadLine)
	g.line("__wisp_read_line")
	lineTemp := g.newTemp()
	g.line("%s=\"$__ret\"", lineTemp)
	eofTemp := g.newTemp()
	g.line("%s=\"$__wisp_rl_eof\"", eofTemp)
	out := g.allocHandle()
	g.line("if [ -n \"$%s\" ]; then", eofTemp)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line("else")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(lineTemp))
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genReadSecret lowers read_secret(prompt) -> Optional[string]. Mirrors genReadLine
// (builds the Optional from __ret + the shared __wisp_rl_eof, copied immediately) but
// passes the prompt as the helper's $1.
func (g *gen) genReadSecret(n *ast.CallExpr, args []ast.Expr) atom {
	prompt := g.word(g.genExpr(args[0]))
	g.use(runtime.ReadSecret)
	g.line("__wisp_read_secret %s", prompt)
	lineTemp := g.newTemp()
	g.line("%s=\"$__ret\"", lineTemp)
	eofTemp := g.newTemp()
	g.line("%s=\"$__wisp_rl_eof\"", eofTemp)
	out := g.allocHandle()
	g.line("if [ -n \"$%s\" ]; then", eofTemp)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line("else")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(lineTemp))
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genRunFull lowers run_full(argv) -> RunResult. The helper writes
// __wisp_rf_stdout, __wisp_rf_stderr, __wisp_rf_code. After the guard, allocates
// the handle and copies the globals into per-handle vars before any subsequent
// run_full could overwrite them.
func (g *gen) genRunFull(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.RunFull)
	g.line("__wisp_run_full %s \"$%s\"", g.posLiteral(n.Pos()), id)
	g.guardAfterSpill()
	hid := g.allocHandle()
	g.setRunResultFields(hid)
	return varAtom(hid)
}

// genRunInput lowers run_input(argv, stdin) -> string. Mirrors genRun, passing
// the stdin string as the helper's 3rd arg (fed to the child via printf %s).
func (g *gen) genRunInput(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	stdin := g.word(g.genExpr(args[1]))
	g.use(runtime.RunInput)
	g.line("__wisp_run_input %s \"$%s\" %s", g.posLiteral(n.Pos()), id, stdin)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genRunInputFull lowers run_input_full(argv, stdin) -> RunResult. Mirrors
// genRunFull, passing the stdin as the helper's 3rd arg.
func (g *gen) genRunInputFull(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	stdin := g.word(g.genExpr(args[1]))
	g.use(runtime.RunInputFull)
	g.line("__wisp_run_input_full %s \"$%s\" %s", g.posLiteral(n.Pos()), id, stdin)
	g.guardAfterSpill()
	hid := g.allocHandle()
	g.setRunResultFields(hid)
	return varAtom(hid)
}

// genPipe lowers pipe(stages) -> RunResult. The located helper sets the
// __wisp_rf_{stdout,stderr,code} globals (like run_full); we allocate a RunResult
// handle and store them. Identical handle-construction shape to genRunFull.
func (g *gen) genPipe(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.Pipe)
	g.line("__wisp_pipe %s \"$%s\"", g.posLiteral(n.Pos()), id)
	g.guardAfterSpill()
	hid := g.allocHandle()
	g.setRunResultFields(hid)
	return varAtom(hid)
}

// genSpawn lowers spawn(argv) -> Process. The located helper sets globals
// __wisp_sp_{pid,wrap,out,err,pidf,done}; we allocate a Process handle and store
// each plus state="running". Mirrors genRunFull's handle-construction shape.
func (g *gen) genSpawn(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.Spawn)
	g.line("__wisp_spawn %s \"$%s\"", g.posLiteral(n.Pos()), id)
	g.guardAfterSpill()
	hid := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+hid+"}_pid", varAtom("__wisp_sp_pid"))
	g.setHandleVar("__wisp_s_${"+hid+"}_wrap", varAtom("__wisp_sp_wrap"))
	g.setHandleVar("__wisp_s_${"+hid+"}_out", varAtom("__wisp_sp_out"))
	g.setHandleVar("__wisp_s_${"+hid+"}_err", varAtom("__wisp_sp_err"))
	g.setHandleVar("__wisp_s_${"+hid+"}_pidf", varAtom("__wisp_sp_pidf"))
	g.setHandleVar("__wisp_s_${"+hid+"}_done", varAtom("__wisp_sp_done"))
	g.setHandleVar("__wisp_s_${"+hid+"}_state", litAtom("'running'"))
	return varAtom(hid)
}

// genWait lowers wait(p) -> RunResult. The helper returns the RunResult handle
// id in __ret (idempotent: cached after the first call). No located abort.
func (g *gen) genWait(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.Wait)
	g.line("__wisp_wait \"$%s\"", id)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genIsDone lowers is_done(p) -> bool (total; __ret=true/false).
func (g *gen) genIsDone(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.IsDone)
	g.line("__wisp_is_done \"$%s\"", id)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genSignal lowers signal(p, sig) -> void (total; no located abort). sig is the
// checker-validated literal, emitted as a %q word (mirrors genOnSignal).
func (g *gen) genSignal(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	sig, ok := stringLitText(args[1])
	if !ok {
		panic("genSignal: arg1 is not a string literal (checker invariant violated)")
	}
	g.use(runtime.Signal)
	g.line("__wisp_signal \"$%s\" %q", id, sig)
	return litAtom("''")
}

// genWaitAny lowers wait_any(ps, poll_secs) -> Process (located; __ret = the
// first-done Process handle id). arg0 is the Process[] array handle id; arg1 the
// poll int.
func (g *gen) genWaitAny(n *ast.CallExpr, args []ast.Expr) atom {
	aid := g.spillToTemp(g.genExpr(args[0]))
	poll := g.spillToTemp(g.genExpr(args[1]))
	g.use(runtime.WaitAny)
	g.line("__wisp_wait_any %s \"$%s\" \"$%s\"", g.posLiteral(n.Pos()), aid, poll)
	g.guardAfterSpill()
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genRunEnvStatus lowers run_env_status(argv, env) -> int. Same shape as genRunEnv
// but hands the new env-prefixed argv to the EXISTING __wisp_run_status path (bare
// execution, returns child exit code, no abort on nonzero). The shared builder's
// empty-argv and invalid-name aborts still apply before the run_status call.
func (g *gen) genRunEnvStatus(n *ast.CallExpr, args []ast.Expr) atom {
	argvID := g.spillToTemp(g.genExpr(args[0]))
	envID := g.spillToTemp(g.genExpr(args[1]))
	g.use(runtime.RunEnv)
	newID := g.allocHandle()
	pos := g.posLiteral(n.Pos())
	g.line("__wisp_run_env_argv %s \"$%s\" \"$%s\" \"$%s\"", pos, newID, argvID, envID)
	g.guardAfterSpill()
	g.use(runtime.RunStatus)
	g.line("__wisp_run_status %s \"$%s\"", pos, newID)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genRunEnvFull lowers run_env_full(argv, env) -> RunResult. Same shape as genRunEnv
// but hands the new env-prefixed argv to the EXISTING __wisp_run_full path (captures
// stdout/stderr/code into __wisp_rf_* globals, no abort on nonzero). After guarding,
// constructs the RunResult handle exactly as genRunFull does.
func (g *gen) genRunEnvFull(n *ast.CallExpr, args []ast.Expr) atom {
	argvID := g.spillToTemp(g.genExpr(args[0]))
	envID := g.spillToTemp(g.genExpr(args[1]))
	g.use(runtime.RunEnv)
	newID := g.allocHandle()
	pos := g.posLiteral(n.Pos())
	g.line("__wisp_run_env_argv %s \"$%s\" \"$%s\" \"$%s\"", pos, newID, argvID, envID)
	g.guardAfterSpill()
	g.use(runtime.RunFull)
	g.line("__wisp_run_full %s \"$%s\"", pos, newID)
	g.guardAfterSpill()
	hid := g.allocHandle()
	g.setRunResultFields(hid)
	return varAtom(hid)
}

// genExit lowers exit(code) to a direct `exit <code>`: it is a process exit, not
// a fault, so no located prefix and no helper. The int operand is evaluated to a
// word (digits for a literal, a quoted expansion for a variable; every int value
// is [+-]?[0-9]+, so the word is safe).
func (g *gen) genExit(args []ast.Expr) atom {
	code := g.genExpr(args[0])
	g.line("exit %s", g.word(code))
	return litAtom("''")
}

// genOnExit lowers on_exit(handler) -> void. The handler is a funcref; its
// value lowers to litAtom(fr.Mangled) -- a bare mangled-name WORD (already
// returned by genExpr for a FuncRef ident). We read that word DIRECTLY and
// pass it to the total __wisp_on_exit helper as a positional argument (NOT
// spilled to a $temp -- a function-local temp would not survive into a
// late-firing trap). The helper installs an exit-code-preserving EXIT trap.
func (g *gen) genOnExit(args []ast.Expr) atom {
	handler := g.genExpr(args[0]) // always a litAtom(mangled) for a funcref
	g.use(runtime.OnExit)
	g.line("__wisp_on_exit %s", handler.name)
	return litAtom("''")
}

// genOnSignal lowers on_signal(sig, handler) -> void. TOTAL: no located abort,
// no __wisp_fail, no errMode. Both operands are compile-time-known inert words:
// the handler funcref lowers to its bare mangled-name WORD (litAtom(fr.Mangled),
// read directly like genOnExit -- NOT spilled to a $temp that would not survive
// into a late-firing trap), and sig is the checker-validated allowlist literal
// (an [A-Z0-9]+ word). Emit `__wisp_on_signal <mangled> <sig>`; the helper
// installs `trap "<mangled>" "<sig>"`.
func (g *gen) genOnSignal(args []ast.Expr) atom {
	sig, ok := stringLitText(args[0]) // validated at check time; always a constant literal here
	if !ok {
		panic("genOnSignal: arg0 is not a string literal (checker invariant violated)")
	}
	handler := g.genExpr(args[1]) // always a litAtom(mangled) for a funcref
	g.use(runtime.OnSignal)
	g.line("__wisp_on_signal %s %q", handler.name, sig)
	return litAtom("''")
}

// stringLitText returns the constant text of a string literal with no
// interpolation (all text parts), and ok=true; ok=false for a non-StringLit or
// an interpolated literal. Mirrors the checker's literal-sig predicate so
// genOnSignal can re-extract the already-validated signal name.
func stringLitText(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.StringLit)
	if !ok {
		return "", false
	}
	var text string
	for _, p := range lit.Parts {
		if !p.IsText() {
			return "", false
		}
		text += p.Text
	}
	return text, true
}

// genAbs lowers abs(x). int: the __wisp_abs_int helper, which aborts located on
// abs(INT_MIN) (the most-negative value has no positive counterpart, so the
// $(( )) negate would overflow and zsh mis-evaluates it). float: the __wisp_fabs
// awk helper (finiteness-validated, located). Both are located/fallible.
func (g *gen) genAbs(n *ast.CallExpr, args []ast.Expr) atom {
	if g.info.Types[args[0]] == types.Float {
		return g.genLocatedHelperCall(runtime.FAbs, "__wisp_fabs", n, args)
	}
	return g.genLocatedHelperCall(runtime.AbsInt, "__wisp_abs_int", n, args)
}

// genMinMax lowers min/max(a, b). It chooses an operand and returns it UNCHANGED
// (the chosen operand's original atom): no arithmetic/awk reformat of the value,
// so a float result carries no new validity obligation. int comparison uses a
// numeric `[ ]`; float comparison uses __wisp_fcmp (the M3 exit-status compare)
// to decide which operand to copy.
func (g *gen) genMinMax(args []ast.Expr, isMin bool) atom {
	a := g.spillToTemp(g.genExpr(args[0]))
	b := g.spillToTemp(g.genExpr(args[1]))
	t := g.newTemp()
	if g.info.Types[args[0]] == types.Float {
		// Pick a when (isMin ? a<=b : a>=b), else b; copy the chosen operand
		// unchanged. The op token is compiler-fixed.
		op := "ge"
		if isMin {
			op = "le"
		}
		g.use(runtime.FCmp)
		g.line("__wisp_fcmp %s %s \"$%s\" \"$%s\"", g.posLiteral(args[0].Pos()), shellSingleQuote(op), a, b)
		g.line("if [ \"$__ret\" = true ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", t, a, t, b)
		return varAtom(t)
	}
	op := "-ge"
	if isMin {
		op = "-le"
	}
	g.line("if [ \"$%s\" %s \"$%s\" ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", a, op, b, t, a, t, b)
	return varAtom(t)
}

// useBuiltinFuncrefExtras marks any sentinel dependency a builtin funcref
// wrapper needs beyond its own registry snippet. Only program_path's wrapper
// wraps a helper that reads the Arg0 sentinel global, which (being a sentinel,
// not a registry dependency) is invisible to EmitMode's deps-closure walk and
// so must be g.use'd explicitly wherever the wrapper is referenced as a value.
func (g *gen) useBuiltinFuncrefExtras(mangled string) {
	if mangled == "__wisp_builtin_program_path" {
		g.use(runtime.Arg0)
	}
}

// genHelperCall evaluates the args, calls the named non-fallible prelude helper
// (marking it used for tree-shaking), and reads __ret into a fresh temp. n is
// the call site (unused here; kept for signature parity with the located form).
func (g *gen) genHelperCall(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	_ = n
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s%s", fn, joinWords(words))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genLocatedHelperCall is like genHelperCall but for a helper whose body can
// abort. It prepends the re-encoded call-site position as the helper's leading
// <pos> argument, which the helper forwards to __wisp_fail (spec section 4). The
// args are evaluated AFTER the position literal (a constant token), preserving
// left-to-right argument evaluation order.
func (g *gen) genLocatedHelperCall(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	pos := g.posLiteral(n.Pos())
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s %s%s", fn, pos, joinWords(words))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	// A fallible helper may have set pending (mode-aware __wisp_fail); guard so
	// the rest of the statement's evaluation is skipped on a fault (M5).
	g.guardAfterSpill()
	return varAtom(t)
}

// casePattern renders a switch case value as a POSIX `case` pattern that matches
// it LITERALLY (spec section 9.4). Single-quote wrapping makes every pattern
// metacharacter (* ? [ ] \ and the | separator) inert and survives the shell's
// quote-removal step. The decoded literal bytes are re-encoded, never pasted.
func (g *gen) casePattern(v ast.Expr) string {
	// Every accepted case value is a constant expression whose canonical folded
	// value the checker recorded in FoldedValues (int/string/bool literals, the
	// reserved consts stdout/stderr folding to 1/2, const refs, and folded
	// operator exprs). Emit that folded value so the pattern matches the
	// subject's printed form and stays consistent with the checker's
	// folded-value duplicate detection.
	if fv, ok := g.info.FoldedValues[v]; ok && fv != nil {
		return casePatternFromFolded(fv)
	}
	// Unreachable for any accepted program: every case value is a type-valid
	// constant expression, and checkConstExpr records a non-nil FoldedValues
	// entry for each (stmt.go:819 -> const.go:42-46), so the guard above always
	// fires. Any node reaching here is checker/codegen drift -- fail loud.
	panic(fmt.Sprintf("casePattern: case value %T lacks a non-nil folded value (checker/codegen drift)", v))
}

// casePatternFromFolded emits a POSIX case pattern string from a folded
// compile-time value. Int -> bare decimal (single-quoted). Bool ->
// single-quoted "true"/"false". String -> single-quoted bytes.
func casePatternFromFolded(fv interface{}) string {
	switch cv := fv.(type) {
	case int64:
		return shellSingleQuote(strconv.FormatInt(cv, 10))
	case bool:
		if cv {
			return shellSingleQuote("true")
		}
		return shellSingleQuote("false")
	case string:
		return shellSingleQuote(cv)
	}
	return shellSingleQuote("")
}
