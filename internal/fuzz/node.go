// internal/fuzz/node.go

// Package fuzz is a local-only cross-shell differential fuzzer for the wisp
// compiler. It generates well-typed wisp programs from a seed, compiles each once
// via internal/driver, runs the single artifact under every shell from
// internal/testrunner, and reports any cross-shell divergence or recompile
// instability. It never runs in per-PR CI (see the fuzzshell build tag).
package fuzz

type Kind int

const (
	KInt Kind = iota
	KFloat
	KBool
	KString
	KArray // Elem is the element type
	KDict  // v1 dict keys are always string; Elem is the value type
)

type Type struct {
	Kind Kind
	Elem *Type
}

// Category identifies a payload / language category for coverage witnessing
// (SC-009). Values are bit positions in a CategorySet.
type Category int

const (
	CatIntArith Category = iota
	CatFloatArith
	CatBool
	CatString
	CatArray
	CatDict
	CatControlFlow
	CatCmdSubst
	CatNewline
	CatFormatPct
	CatBackslash
	CatLeadDash
	CatGlob
	CatArithEdge
	numCategories
)

// pureCoreCategories and payloadCategories partition the witnessed categories so
// tests and the CLI report can assert each group (SC-009).
var pureCoreCategories = []Category{CatIntArith, CatFloatArith, CatBool, CatString, CatArray, CatDict, CatControlFlow}
var payloadCategories = []Category{CatCmdSubst, CatNewline, CatFormatPct, CatBackslash, CatLeadDash, CatGlob, CatArithEdge}

type CategorySet uint64

func (s *CategorySet) add(c Category) { *s |= 1 << uint(c) }

// Has is exported so cmd/wisp-fuzz (package main) can render the SC-009 coverage
// report; add stays unexported (only the generator sets categories).
func (s CategorySet) Has(c Category) bool { return s&(1<<uint(c)) != 0 }

// AllCategories is every witnessed category in report order (pure-core then
// payload), for the SC-009 coverage report and its tests.
var AllCategories = append(append([]Category{}, pureCoreCategories...), payloadCategories...)

// String names a category for the SC-009 coverage report (the observable witness).
func (c Category) String() string {
	switch c {
	case CatIntArith:
		return "int-arith"
	case CatFloatArith:
		return "float-arith"
	case CatBool:
		return "bool"
	case CatString:
		return "string"
	case CatArray:
		return "array"
	case CatDict:
		return "dict"
	case CatControlFlow:
		return "control-flow"
	case CatCmdSubst:
		return "cmd-subst"
	case CatNewline:
		return "newline"
	case CatFormatPct:
		return "format-pct"
	case CatBackslash:
		return "backslash"
	case CatLeadDash:
		return "leading-dash"
	case CatGlob:
		return "glob"
	case CatArithEdge:
		return "arith-edge"
	}
	return "cat?"
}

// FamilySet is a set of witnessed admitted-builtin families (SC-009 requires each
// FAMILY, not one coarse bucket).
type FamilySet map[string]bool

func (s FamilySet) add(fam string) {
	if fam != "" {
		s[fam] = true
	}
}

type Expr interface{ isExpr() }

type IntLit struct{ V string }
type FloatLit struct{ V string }
type BoolLit struct{ V bool }
type StringLit struct{ V string }
type Var struct {
	Name string
	T    Type
}
type Binary struct {
	Op   string
	L, R Expr
	T    Type
}
type ArrayLit struct {
	Elems []Expr
	Elem  Type
}
type DictLit struct {
	Keys []string
	Vals []Expr
	Val  Type
}
type Index struct {
	Base Expr
	Key  Expr
	T    Type
}
type Call struct {
	Builtin string
	Args    []Expr
	T       Type
}

func (*IntLit) isExpr()    {}
func (*FloatLit) isExpr()  {}
func (*BoolLit) isExpr()   {}
func (*StringLit) isExpr() {}
func (*Var) isExpr()       {}
func (*Binary) isExpr()    {}
func (*ArrayLit) isExpr()  {}
func (*DictLit) isExpr()   {}
func (*Index) isExpr()     {}
func (*Call) isExpr()      {}

type Stmt interface{ isStmt() }

type LetStmt struct {
	Name string
	T    Type
	Init Expr
}
type PrintStmt struct{ Arg Expr } // Arg is always a printable scalar
type IfStmt struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

func (*LetStmt) isStmt()   {}
func (*PrintStmt) isStmt() {}
func (*IfStmt) isStmt()    {}

type Program struct {
	Imports []string
	Body    []Stmt
	Cats    CategorySet
	Fams    FamilySet
}

// The following are pure structural helpers over the IR. They live here (the IR
// file, Task 1) because both the generator (gen.go, Task 6) and the shrinker
// (shrink.go, Task 8) consume them, so neither task defines-then-moves them.

// leafOfType returns the minimal well-typed leaf expression of a scalar type:
// the type-preserving base case for shrinking a Call/Binary down to a literal,
// and the safe Optional-unwrap fallback the generator emits.
func leafOfType(t Type) Expr {
	switch t.Kind {
	case KFloat:
		return &FloatLit{V: "0.0"}
	case KBool:
		return &BoolLit{V: false}
	case KString:
		return &StringLit{V: ""}
	default:
		return &IntLit{V: "0"}
	}
}

// nodeCount / nodeCountStmts / stmtNodes / exprNodes count AST nodes. The
// generator's MaxNodes budget (gen.go) reads nodeCountStmts over the growing
// SHAPE body (the witness prelude is appended after and is exempt -- see the
// MaxNodes comment in gen.go); the shrinker (shrink.go) reads nodeCount over a
// whole Program to assert each reduction step strictly shrinks it.
func nodeCount(p *Program) int { return nodeCountStmts(p.Body) }

func nodeCountStmts(ss []Stmt) int {
	n := 0
	for _, s := range ss {
		n += stmtNodes(s)
	}
	return n
}

func stmtNodes(s Stmt) int {
	switch n := s.(type) {
	case *LetStmt:
		return 1 + exprNodes(n.Init)
	case *PrintStmt:
		return 1 + exprNodes(n.Arg)
	case *IfStmt:
		c := 1 + exprNodes(n.Cond)
		for _, t := range n.Then {
			c += stmtNodes(t)
		}
		for _, e := range n.Else {
			c += stmtNodes(e)
		}
		return c
	}
	return 1
}

func exprNodes(e Expr) int {
	switch n := e.(type) {
	case *Binary:
		return 1 + exprNodes(n.L) + exprNodes(n.R)
	case *ArrayLit:
		c := 1
		for _, el := range n.Elems {
			c += exprNodes(el)
		}
		return c
	case *DictLit:
		c := 1
		for _, v := range n.Vals {
			c += exprNodes(v)
		}
		return c
	case *Index:
		return 1 + exprNodes(n.Base) + exprNodes(n.Key)
	case *Call:
		c := 1
		for _, a := range n.Args {
			c += exprNodes(a)
		}
		return c
	default:
		return 1
	}
}
