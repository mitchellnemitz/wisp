// internal/fuzz/value.go
package fuzz

import "strconv"

var intEdges = []string{
	"0", "1", "-1", "2", "-2", "10", "-10",
	"9223372036854775807",  // INT_MAX
	"-9223372036854775808", // INT_MIN (Design B: emitted deliberately; oracle carves zsh)
}

func genIntValue(r *RNG, cats *CategorySet) Expr {
	if r.Intn(2) == 0 {
		cats.add(CatArithEdge)
		return &IntLit{V: intEdges[r.Intn(len(intEdges))]}
	}
	return &IntLit{V: strconv.Itoa(r.Intn(2001) - 1000)}
}

// genFloatValue emits a float in a modest range with a fractional part so the
// token lexes as a float. Floats render via a fixed LC_ALL=C template in codegen;
// the fuzzer confirms shell-agnosticism (FR-005/FR-020).
func genFloatValue(r *RNG) Expr {
	return &FloatLit{V: strconv.Itoa(r.Intn(1000)-500) + "." + pad6(r.Intn(1000000))}
}

func pad6(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

func genBoolValue(r *RNG) Expr { return &BoolLit{V: r.Bool()} }

var payloadFragments = []struct {
	cat   Category
	frags []string
}{
	{CatCmdSubst, []string{"$(echo x)", "`id`", "${HOME}"}},
	{CatNewline, []string{"\n", "\t", "  ", "\n\n"}},
	{CatFormatPct, []string{"%s", "%d", "%%", "%n"}},
	{CatBackslash, []string{"\\", "\\n", "\\t", "\\\\"}},
	{CatLeadDash, []string{"-n", "--", "-e"}},
	{CatGlob, []string{"*", "?", "[a-z]", "*.sh"}},
}

func genStringValue(r *RNG, cats *CategorySet) Expr {
	cats.add(CatString)
	n := 1 + r.Intn(3)
	var s string
	for i := 0; i < n; i++ {
		if r.Intn(3) == 0 {
			s += "abc" + strconv.Itoa(r.Intn(100))
			continue
		}
		p := payloadFragments[r.Intn(len(payloadFragments))]
		cats.add(p.cat)
		s += p.frags[r.Intn(len(p.frags))]
	}
	return &StringLit{V: s}
}
