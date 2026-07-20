package types

import (
	"math/big"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// constIntProbe attempts to evaluate e as a compile-time-constant integer
// WITHOUT raising any diagnostic and WITHOUT writing anything to types.Info.
// It returns (value, true) when e folds to an int64 in the wisp int range, else
// (0, false).
//
// This is the side-effect-free is-constant-int probe required by the
// builtin/operator integer-argument domain checks (spec 2.3). Neither existing
// fold entry point is reusable here: checkConstExpr/foldConst emit diagnostics
// and mutate types.Info, and FoldedInt reads info.FoldedValues which are not
// populated for ordinary builtin/operator argument expressions. The probe
// deliberately does NOT resolve identifiers or const references (that would
// require scope state and would write info.Uses); such arguments are treated as
// not-constant, leaving the runtime guard as the sole enforcement. This is a
// sound under-approximation that never produces a false rejection.
func constIntProbe(e ast.Expr) (int64, bool) {
	switch n := e.(type) {
	case *ast.IntLit:
		v, err := parseWispInt(n.Raw, false)
		if err != nil {
			return 0, false
		}
		return v, true

	case *ast.UnaryExpr:
		// Unary minus directly over an int literal: parse the signed magnitude so
		// INT_MIN (-9223372036854775808) folds (its magnitude alone is out of the
		// positive int range). Mirrors foldUnary's special case.
		if n.Op == token.Minus {
			if il, ok := n.X.(*ast.IntLit); ok {
				v, err := parseWispInt(il.Raw, true)
				if err != nil {
					return 0, false
				}
				return v, true
			}
		}
		xv, ok := constIntProbe(n.X)
		if !ok {
			return 0, false
		}
		if n.Op == token.Minus {
			if xv == wispIntMin { // negating INT_MIN overflows the int range
				return 0, false
			}
			return -xv, true
		}
		return 0, false

	case *ast.BinaryExpr:
		lv, lok := constIntProbe(n.L)
		rv, rok := constIntProbe(n.R)
		if !lok || !rok {
			return 0, false
		}
		return foldIntArithProbe(lv, rv, n.Op)

	default:
		return 0, false
	}
}

// foldIntArithProbe evaluates an int binary arithmetic op purely, returning
// (result, true) on success or (0, false) on a non-arithmetic op, a divide- or
// modulo-by-zero, or an int64-range overflow. It mirrors the big.Int overflow
// approach of foldIntArith in const.go (which takes a *checker and rune op codes
// and emits c.errf on overflow); this probe is receiver-free, takes token.Kind,
// and returns false instead of raising, per the side-effect-free requirement.
func foldIntArithProbe(a, b int64, op token.Kind) (int64, bool) {
	ba := new(big.Int).SetInt64(a)
	bb := new(big.Int).SetInt64(b)
	var r big.Int
	switch op {
	case token.Plus:
		r.Add(ba, bb)
	case token.Minus:
		r.Sub(ba, bb)
	case token.Star:
		r.Mul(ba, bb)
	case token.Slash:
		if b == 0 {
			return 0, false
		}
		r.Quo(ba, bb) // truncating toward zero, matching POSIX $(( ))
	case token.Percent:
		if b == 0 {
			return 0, false
		}
		r.Rem(ba, bb)
	default:
		// Bitwise and all non-arithmetic operators are not folded by the probe.
		// The in-scope argument domains never require them, and a non-fold is a
		// sound under-approximation.
		return 0, false
	}
	if r.Cmp(new(big.Int).SetInt64(wispIntMax)) > 0 || r.Cmp(new(big.Int).SetInt64(wispIntMin)) < 0 {
		return 0, false
	}
	return r.Int64(), true
}

// rejectConstDivByZero emits a located "division by zero" error when the divisor
// of an int `/` or `%` folds to the constant 0. This mirrors the runtime guard
// (codegen expr.go:456, "division by zero" for both operators) at compile time
// for the non-const case (spec construct #1/#2, R5/E1). The in-scope reject set
// is exactly {b == 0}; the INT_MIN/-1 overflow case is out of scope (spec 2.2).
func (c *checker) rejectConstDivByZero(divisor ast.Expr) {
	if v, ok := constIntProbe(divisor); ok && v == 0 {
		c.errf(divisor.Pos(), "division by zero")
	}
}

// builtinIntArgDomain describes one compile-time integer-argument domain check
// for a builtin: which argument to inspect (0-based) and which folded values to
// reject, with the located diagnostic message. The message mirrors the shipped
// runtime guard string (spec Section 4).
// msg is a Printf format string with exactly one %s verb, filled with the
// call's dispName (the module-qualified spelling for a delegate call, or the
// flat name for a flat call) — never a fully-baked string.
type builtinIntArgDomain struct {
	argIndex int
	reject   func(int64) bool
	msg      string
}

// builtinIntArgDomains maps a builtin name to its integer-argument domain
// checks. Multiple entries (gcd) are checked in order; the FIRST offending
// argument yields the single diagnostic. The reject sets and messages are the
// in-scope subset from spec Section 2.1 / Section 4 (verified against the
// shipped guards). Constructs not listed here (div/mod #1/#2, array index #11)
// are handled at their own checker sites.
var builtinIntArgDomains = map[string][]builtinIntArgDomain{
	"repeat":       {{1, func(v int64) bool { return v < 0 }, "%s(): negative count"}},
	"random":       {{0, func(v int64) bool { return v <= 0 }, "%s: max must be positive"}},
	"format_float": {{1, func(v int64) bool { return v < 0 }, "%s: decimals must be >= 0"}},
	"chr":          {{0, func(v int64) bool { return v < 1 || v > 255 }, "%s(): code out of range 1-255"}},
	"sleep":        {{0, func(v int64) bool { return v < 0 }, "%s: negative duration"}},
	"wait_any":     {{1, func(v int64) bool { return v < 0 }, "%s: poll_secs must be >= 0"}},
	"remove_at":    {{1, func(v int64) bool { return v < 0 }, "%s: index out of range"}},
	"insert_at":    {{1, func(v int64) bool { return v < 0 }, "%s: index out of range"}},
	"abs":          {{0, func(v int64) bool { return v == wispIntMin }, "%s(): integer overflow"}},
	"gcd": {
		{0, func(v int64) bool { return v == wispIntMin }, "%s(): integer overflow"},
		{1, func(v int64) bool { return v == wispIntMin }, "%s(): integer overflow"},
	},
}

// checkBuiltinArgDomains rejects a builtin call whose designated integer-scalar
// argument is a compile-time-constant in the construct's in-scope reject set
// (spec Section 2.1). It is a no-op for builtins not in the table, for
// non-constant arguments, and when the argument index is absent (an arity error
// is reported elsewhere). For builtins with multiple checked positions (gcd),
// only the first offending argument is reported (spec AC1).
func (c *checker) checkBuiltinArgDomains(n *ast.CallExpr, name, dispName string) {
	checks, ok := builtinIntArgDomains[name]
	if !ok {
		return
	}
	for _, chk := range checks {
		if chk.argIndex >= len(n.Args) {
			continue
		}
		arg := n.Args[chk.argIndex]
		if v, ok := constIntProbe(arg); ok && chk.reject(v) {
			c.errf(arg.Pos(), chk.msg, dispName)
			return
		}
	}
}
