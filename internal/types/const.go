package types

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// FoldedFloat carries the raw decimal text of a float-typed constant. Float
// constants are not folded arithmetically (spec section 5), so the value is
// just enough to re-emit the literal at codegen sites: a bare, injection-safe
// word matching the float-validity invariant [+-]?[0-9]+(\.[0-9]+)?, lowered
// exactly like a normal FloatLit. It is a distinct type so codegen's folded
// value switch never confuses it with a string const (which is single-quoted).
type FoldedFloat struct{ Raw string }

// negateFloatRaw toggles a leading minus on a float literal's raw text, used to
// fold unary minus over a float-typed constant expression.
func negateFloatRaw(raw string) string {
	if strings.HasPrefix(raw, "-") {
		return raw[1:]
	}
	return "-" + raw
}

// int64 range matching __wisp_int in internal/runtime/prelude.go.
const (
	wispIntMax = int64(9223372036854775807)
	wispIntMin = int64(-9223372036854775808)
)

// checkConstExpr validates that e is a constant expression and records the
// resolved type in Info.Types. It also stores the folded compile-time value in
// Info.FoldedValues (int64 for int, bool for bool, string for string,
// FoldedFloat for float). A non-constant expression is an error and returns
// Invalid.
func (c *checker) checkConstExpr(e ast.Expr) Type {
	t, v := c.foldConst(e)
	c.info.Types[e] = t
	if t != Invalid {
		c.info.FoldedValues[e] = v
	}
	return t
}

// foldConst evaluates e as a constant expression and returns its type and
// folded value. Float literals return (Float, FoldedFloat{raw}) -- no
// arithmetic folding, just the raw text for codegen re-emission.
// On error it records the diagnostic, returns (Invalid, nil), and does NOT
// store anything in FoldedValues.
func (c *checker) foldConst(e ast.Expr) (Type, interface{}) {
	switch n := e.(type) {

	case *ast.IntLit:
		v, err := parseWispInt(n.Raw, false)
		if err != nil {
			c.errf(n.LitPos, "integer literal out of range: %q", n.Raw)
			return Invalid, nil
		}
		return Int, v

	case *ast.FloatLit:
		// Float literals are valid constant expressions but cannot participate
		// in arithmetic (spec §5). The folded value carries the raw text so
		// codegen re-emits it as a bare float-literal word.
		if err := floatLitInDomain(n.Raw); err != nil {
			c.errf(n.LitPos, "float literal out of domain: %q", n.Raw)
			return Invalid, nil
		}
		return Float, FoldedFloat{Raw: n.Raw}

	case *ast.BoolLit:
		return Bool, n.Value

	case *ast.StringLit:
		for _, part := range n.Parts {
			if !part.IsText() {
				c.errf(e.Pos(), "a constant expression may not contain interpolation")
				return Invalid, nil
			}
		}
		// Concatenate all text parts into a single string value.
		var s string
		for _, p := range n.Parts {
			s += p.Text
		}
		return String, s

	case *ast.Ident:
		if n.Name == "Some" || n.Name == "None" || n.Name == "Ok" || n.Name == "Err" {
			c.errf(n.NamePos, "%q is not a constant expression", n.Name)
			return Invalid, nil
		}
		if n.Name == "stdout" {
			return Int, int64(1)
		}
		if n.Name == "stderr" {
			return Int, int64(2)
		}
		// Const-reference resolution hook (wired by Task 3).
		if c.constResolver != nil {
			if v, t, ok := c.constResolver(n.Name); ok {
				if t != Invalid {
					// Record info.Uses so codegen genIdent and LSP find-references
					// can locate the declaration Var for this const reference. This
					// runs in BOTH phases: during body checking c.lookup finds a
					// local const; during top-level folding (curFunc == nil) the
					// scope stack is empty, so resolution falls to c.cur.topConsts,
					// which is how a reference inside a top-level const initializer
					// (const B = A + 1) gets recorded for LSP navigation.
					if cv := c.lookup(n.Name); cv != nil && cv.IsConst {
						c.info.Uses[n] = cv
					} else if tv := c.cur.topConsts[n.Name]; tv != nil {
						c.info.Uses[n] = tv
					}
				}
				return t, v
			}
		}
		c.errf(n.NamePos, "a constant expression may not reference a variable (%q)", n.Name)
		return Invalid, nil

	case *ast.UnaryExpr:
		return c.foldUnary(n)

	case *ast.BinaryExpr:
		return c.foldBinary(n)

	case *ast.FieldAccess:
		// Enum variant access `Color.Red` folds UNCONDITIONALLY (R3), not gated by
		// foldAllowsQualified, so a variant is a valid constant in const/final
		// initializers and switch cases. The base must name an enum type (not be
		// shadowed by a local or namespace); an unknown variant is a located error.
		if tok, ei, ok := c.enumTypeOfBase(n); ok {
			if v, found := ei.value(n.Field); found {
				c.info.Types[n] = tok
				return tok, v
			}
			c.errf(n.DotPos, "enum %q has no variant %q", ei.Name, n.Field)
			return Invalid, nil
		}
		// A cross-module `ns.NAME` const is permitted in a const-expr ONLY in a
		// default-argument context (foldAllowsQualified, AC3/R10). When the base is a
		// namespace alias there, resolveQualifiedConst fully handles it (resolve or
		// the right diagnostic) and returns handled == true, so we return its
		// (type, value) directly. In a const INITIALIZER (foldAllowsQualified false),
		// or for a base that is not a namespace alias, fall through to the default
		// arm's "not a constant expression" diagnostic (file-local rule, AC6).
		if c.foldAllowsQualified {
			if field, modid, ok := c.qualifiedNsTarget(n); ok {
				t, v, _ := c.resolveQualifiedConst(n, field, modid, Invalid)
				return t, v
			}
		}
		c.errf(e.Pos(), "the value must be a constant expression (a literal, an operation on constants, or a reference to an earlier constant)")
		return Invalid, nil

	default:
		c.errf(e.Pos(), "the value must be a constant expression (a literal, an operation on constants, or a reference to an earlier constant)")
		return Invalid, nil
	}
}

func (c *checker) foldUnary(n *ast.UnaryExpr) (Type, interface{}) {
	// Special case: unary minus directly over an integer literal. The lexer/parser
	// produces `-N` as Minus over an unsigned-magnitude IntLit, so the magnitude
	// alone may be out of the positive range (e.g. 9223372036854775808, whose only
	// valid form is the negated min int64). Parse the literal's magnitude WITH the
	// sign applied so -9223372036854775808 is accepted while any other out-of-range
	// magnitude (positive or below min) is still rejected.
	if n.Op == token.Minus {
		if il, ok := n.X.(*ast.IntLit); ok {
			v, err := parseWispInt(il.Raw, true)
			if err != nil {
				c.errf(il.LitPos, "integer literal out of range: %q", "-"+il.Raw)
				return Invalid, nil
			}
			// Record the operand node's type so downstream consumers see it as int.
			c.info.Types[n.X] = Int
			return Int, v
		}
	}

	xt, xv := c.foldConst(n.X)
	c.info.Types[n.X] = xt
	if xt != Invalid {
		c.info.FoldedValues[n.X] = xv
	}
	if xt == Invalid {
		return Invalid, nil
	}

	switch n.Op {
	case token.Minus:
		switch xt {
		case Int:
			iv := xv.(int64)
			// Negate with overflow check: only wispIntMin has no positive counterpart.
			if iv == wispIntMin {
				c.errf(n.OpPos, "constant integer out of range (overflow)")
				return Invalid, nil
			}
			return Int, -iv
		case Float:
			// -<float const-expr> is valid; carry the negated raw text so codegen
			// re-emits it as a signed float-literal word.
			if ff, ok := xv.(FoldedFloat); ok {
				return Float, FoldedFloat{Raw: negateFloatRaw(ff.Raw)}
			}
			return Float, nil
		default:
			c.errf(n.OpPos, "unary - requires an int or float operand")
			return Invalid, nil
		}

	case token.Bang:
		if xt != Bool {
			c.errf(n.OpPos, "unary ! requires a bool operand")
			return Invalid, nil
		}
		return Bool, !xv.(bool)

	default:
		c.errf(n.OpPos, "a constant expression allows only unary - (on an int or float) or unary ! (on a bool)")
		return Invalid, nil
	}
}

func (c *checker) foldBinary(n *ast.BinaryExpr) (Type, interface{}) {
	op := n.Op

	lt, lv := c.foldConst(n.L)
	c.info.Types[n.L] = lt
	if lt != Invalid {
		c.info.FoldedValues[n.L] = lv
	}

	// Boolean operators short-circuit, matching runtime semantics (spec): the
	// right operand is evaluated only when the left does not decide the result.
	// An unreachable right operand therefore never reports a fold-time error --
	// `false && (10 / 0 == 0)` folds to false with no divide-by-zero.
	if op == token.AndAnd || op == token.OrOr {
		if lt == Invalid {
			return Invalid, nil
		}
		if lt != Bool {
			c.errf(n.OpPos, "%s requires bool operands", op)
			return Invalid, nil
		}
		lb := lv.(bool)
		if op == token.AndAnd && !lb {
			return Bool, false
		}
		if op == token.OrOr && lb {
			return Bool, true
		}
		// Left does not decide the result: the value is the right operand, which
		// must also be a bool.
		rt, rv := c.foldConst(n.R)
		c.info.Types[n.R] = rt
		if rt == Invalid {
			return Invalid, nil
		}
		c.info.FoldedValues[n.R] = rv
		if rt != Bool {
			c.errf(n.OpPos, "%s requires bool operands", op)
			return Invalid, nil
		}
		return Bool, rv.(bool)
	}

	// All other operators evaluate both operands.
	rt, rv := c.foldConst(n.R)
	c.info.Types[n.R] = rt
	if rt != Invalid {
		c.info.FoldedValues[n.R] = rv
	}

	if lt == Invalid || rt == Invalid {
		return Invalid, nil
	}

	// Float arithmetic in a const-expr is a compile error (spec §5).
	if lt == Float || rt == Float {
		c.errf(n.OpPos, "float arithmetic is not allowed in a constant expression")
		return Invalid, nil
	}

	// Operands must share the same type for all operators.
	if lt != rt {
		c.errf(n.OpPos, "mismatched types in constant expression: %s and %s", lt, rt)
		return Invalid, nil
	}

	// Equality / inequality work on int, bool, string.
	if op == token.Eq || op == token.Neq {
		var eq bool
		switch lt {
		case Int:
			eq = lv.(int64) == rv.(int64)
		case Bool:
			eq = lv.(bool) == rv.(bool)
		case String:
			eq = lv.(string) == rv.(string)
		default:
			c.errf(n.OpPos, "%s is not supported for type %s in a constant expression", op, lt)
			return Invalid, nil
		}
		if op == token.Neq {
			eq = !eq
		}
		return Bool, eq
	}

	// Ordered comparisons: int and string only.
	if op == token.Lt || op == token.Lte || op == token.Gt || op == token.Gte {
		var cmp int // -1/0/1
		switch lt {
		case Int:
			a, b := lv.(int64), rv.(int64)
			if a < b {
				cmp = -1
			} else if a > b {
				cmp = 1
			}
		case String:
			a, b := lv.(string), rv.(string)
			if a < b {
				cmp = -1
			} else if a > b {
				cmp = 1
			}
		default:
			c.errf(n.OpPos, "%s is not supported for type %s in a constant expression", op, lt)
			return Invalid, nil
		}
		var result bool
		switch op {
		case token.Lt:
			result = cmp < 0
		case token.Lte:
			result = cmp <= 0
		case token.Gt:
			result = cmp > 0
		case token.Gte:
			result = cmp >= 0
		}
		return Bool, result
	}

	// Arithmetic and string concat.
	switch op {
	case token.Plus:
		switch lt {
		case Int:
			return foldIntArith(c, n.OpPos, lv.(int64), rv.(int64), '+')
		case String:
			return String, lv.(string) + rv.(string)
		default:
			c.errf(n.OpPos, "+ is not supported for type %s in a constant expression", lt)
			return Invalid, nil
		}
	case token.Minus:
		if lt != Int {
			c.errf(n.OpPos, "- is not supported for type %s in a constant expression", lt)
			return Invalid, nil
		}
		return foldIntArith(c, n.OpPos, lv.(int64), rv.(int64), '-')
	case token.Star:
		if lt != Int {
			c.errf(n.OpPos, "* is not supported for type %s in a constant expression", lt)
			return Invalid, nil
		}
		return foldIntArith(c, n.OpPos, lv.(int64), rv.(int64), '*')
	case token.Slash:
		if lt != Int {
			c.errf(n.OpPos, "/ is not supported for type %s in a constant expression", lt)
			return Invalid, nil
		}
		if rv.(int64) == 0 {
			c.errf(n.OpPos, "constant expression: divide by zero")
			return Invalid, nil
		}
		return foldIntArith(c, n.OpPos, lv.(int64), rv.(int64), '/')
	case token.Percent:
		if lt != Int {
			c.errf(n.OpPos, "%% is not supported for type %s in a constant expression", lt)
			return Invalid, nil
		}
		if rv.(int64) == 0 {
			c.errf(n.OpPos, "constant expression: modulo by zero")
			return Invalid, nil
		}
		return foldIntArith(c, n.OpPos, lv.(int64), rv.(int64), '%')
	default:
		c.errf(n.OpPos, "operator %s is not allowed in a constant expression", op)
		return Invalid, nil
	}
}

// foldIntArith performs op on a and b, checking for int64 range overflow.
func foldIntArith(c *checker, pos token.Position, a, b int64, op rune) (Type, interface{}) {
	// Use big.Int to detect overflow cleanly without relying on two's-complement
	// wrap-around in Go (which would be defined behaviour but misleading here).
	ba := new(big.Int).SetInt64(a)
	bb := new(big.Int).SetInt64(b)
	var result big.Int
	switch op {
	case '+':
		result.Add(ba, bb)
	case '-':
		result.Sub(ba, bb)
	case '*':
		result.Mul(ba, bb)
	case '/':
		result.Quo(ba, bb) // truncating toward zero, matching POSIX $(( ))
	case '%':
		result.Rem(ba, bb)
	}
	// Range check: must fit in [-9223372036854775808, 9223372036854775807].
	bmax := new(big.Int).SetInt64(wispIntMax)
	bmin := new(big.Int).SetInt64(wispIntMin)
	if result.Cmp(bmax) > 0 || result.Cmp(bmin) < 0 {
		c.errf(pos, "constant integer out of range (overflow): %s", result.String())
		return Invalid, nil
	}
	return Int, result.Int64()
}

// parseWispInt parses a decimal string and checks it fits in the wisp int range.
// If neg is true the value is negative (for a unary-minus context over a literal
// whose Raw field is the magnitude without sign).
func parseWispInt(raw string, neg bool) (int64, error) {
	// Use big.Int to handle values near the 64-bit boundary without losing
	// precision in strconv.ParseInt.
	bi, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return 0, fmt.Errorf("not a decimal integer: %q", raw)
	}
	if neg {
		bi.Neg(bi)
	}
	bmax := new(big.Int).SetInt64(wispIntMax)
	bmin := new(big.Int).SetInt64(wispIntMin)
	if bi.Cmp(bmax) > 0 || bi.Cmp(bmin) < 0 {
		return 0, fmt.Errorf("out of range: %q", raw)
	}
	return bi.Int64(), nil
}

// floatLitInDomain reports whether a float literal's raw text denotes a value in
// the runtime finite-float domain (spec R2): the value rendered by %.17g (the
// awk format the float helpers use) must be a finite decimal with no exponent,
// matching what __wisp_ffinite accepts. The runtime (awk %.17g -> __wisp_ffinite)
// is authoritative; this reproduces its decision at compile time. Returns nil if
// in-domain, else a non-nil error.
func floatLitInDomain(raw string) error {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return fmt.Errorf("out of domain")
	}
	s := fmt.Sprintf("%.17g", v)
	// Reject exponent form and any non-finite spelling; %.17g of a finite value
	// is either a plain decimal or an exponent form. The exponent form is the
	// only way a parsed-finite digits.digits value renders out of __wisp_ffinite's
	// accepted shape, but check defensively for inf/nan spellings too.
	if strings.ContainsAny(s, "eE") ||
		strings.Contains(s, "inf") || strings.Contains(s, "Inf") ||
		strings.Contains(s, "nan") || strings.Contains(s, "NaN") {
		return fmt.Errorf("out of domain")
	}
	return nil
}

// FoldedInt returns the folded int64 for an expression previously checked via
// checkConstExpr, and true if it was an int const. Convenience for callers.
func FoldedInt(info *Info, e ast.Expr) (int64, bool) {
	v, ok := info.FoldedValues[e]
	if !ok {
		return 0, false
	}
	iv, ok := v.(int64)
	return iv, ok
}

// FoldedBool returns the folded bool for an expression previously checked via
// checkConstExpr, and true if it was a bool const.
func FoldedBool(info *Info, e ast.Expr) (bool, bool) {
	v, ok := info.FoldedValues[e]
	if !ok {
		return false, false
	}
	bv, ok := v.(bool)
	return bv, ok
}

// FoldedString returns the folded string for an expression previously checked
// via checkConstExpr, and true if it was a string const.
func FoldedString(info *Info, e ast.Expr) (string, bool) {
	v, ok := info.FoldedValues[e]
	if !ok {
		return "", false
	}
	sv, ok := v.(string)
	return sv, ok
}

// FoldedLiteralText returns the canonical text representation of a folded
// constant value for use at codegen sites (case patterns, default-arg inline,
// value-position inline). For int it returns the decimal string; for bool it
// returns "true" or "false"; for string it returns the raw Go string (callers
// are responsible for quoting for their context). Float is not supported here
// (callers should use the raw FloatLit.Raw text). Returns "", false if the
// value is not a recognized fold type.
func FoldedLiteralText(info *Info, e ast.Expr) (string, bool) {
	v, ok := info.FoldedValues[e]
	if !ok {
		return "", false
	}
	switch cv := v.(type) {
	case int64:
		return strconv.FormatInt(cv, 10), true
	case bool:
		if cv {
			return "true", true
		}
		return "false", true
	case string:
		return cv, true
	}
	return "", false
}
