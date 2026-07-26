// internal/fuzz/gen.go
package fuzz

import (
	"fmt"
	"strconv"
)

// GenConfig bounds the program-SHAPE generator (FR-003: modest bounded shapes).
// MaxDepth caps expression nesting; MaxStmts caps the number of random-walk
// statements; MaxNodes is the explicit AST node-count budget FR-003 requires -- and
// it bounds the SHAPE GENERATOR itself (the payload-centric random walk), which is
// the thing FR-003 names. The witness prelude is NOT part of the shape generator: it
// is a FIXED-SIZE, deterministic scaffold (one witness per admitted family + one per
// category -- both finite, known sets) emitted unconditionally first, so its node
// count is a known constant, not a configured variable, and is EXCLUDED from the
// MaxNodes shape budget. MaxNodes stays a true cap on the generator it names (setting
// it below the prelude size can never starve the walk). Total program size is bounded
// and configured but NOT equal to MaxNodes: it is the fixed prelude constant, plus the
// shape walk (<= MaxNodes nodes, plus one overshoot statement), plus the trailing
// value-print scaffold Generate appends (one PrintStmt per scalar variable so every
// generated value reaches stdout for the oracle). That scaffold is itself bounded by
// the walk -- each printed variable is a LetStmt already counted in the walk, so it
// adds at most O(MaxNodes) further print nodes -- so the total is O(prelude + MaxNodes),
// never open-ended (FR-003). DefaultGenConfig's MaxNodes (400) leaves the walk ample room.
type GenConfig struct {
	MaxDepth int
	MaxStmts int
	MaxNodes int
}

func DefaultGenConfig() GenConfig { return GenConfig{MaxDepth: 3, MaxStmts: 6, MaxNodes: 400} }

type genState struct {
	r    *RNG
	cfg  GenConfig
	cats *CategorySet
	fams FamilySet
	imps map[string]bool
	vars []*Var
	next int
}

// Generate builds one well-typed program deterministically from r. It opens with an
// unconditional witness prelude that exercises EVERY admitted family and EVERY
// pure-core/payload category, so SC-009 coverage is guaranteed by construction from
// any single program (no dependence on the random walk or manifest luck) and every
// coverage tag provably reaches stdout -- the FR-003 witness rule (a category counts
// only when a reachable occurrence's value reaches stdout). The prelude is a fixed
// finite scaffold whose nodes are EXCLUDED from the MaxNodes budget; the payload-centric
// random walk -- the shape generator FR-003 bounds -- is what MaxNodes caps.
func Generate(r *RNG, cfg GenConfig) *Program {
	var cats CategorySet
	st := &genState{r: r, cfg: cfg, cats: &cats, fams: FamilySet{}, imps: map[string]bool{}}
	var body []Stmt

	// Unconditional coverage prelude (deterministic, no RNG draw): one printed
	// witness per admitted family, then one printed witness per category.
	for _, s := range st.familyWitnesses() {
		body = appendFlat(body, s)
	}
	for _, s := range st.categoryWitnesses() {
		body = appendFlat(body, s)
	}
	preludeNodes := nodeCountStmts(body) // fixed scaffold size; excluded from the MaxNodes shape budget

	n := 1 + r.Intn(cfg.MaxStmts)
	for i := 0; i < n; i++ {
		if nodeCountStmts(body)-preludeNodes >= cfg.MaxNodes {
			break // FR-003 shape-walk node-count budget reached; stop growing the walk
		}
		body = appendFlat(body, st.genStmt())
	}
	// Trailing value-print scaffold: print every scalar variable so each generated
	// value reaches stdout, where the oracle byte-compares it across shells. This is
	// NOT counted against MaxNodes (it runs after the walk-budget loop), but it is
	// bounded by the walk -- one print per variable, and every variable is a walk
	// LetStmt already counted in MaxNodes -- so total size stays O(prelude + MaxNodes).
	for _, v := range st.vars {
		if isScalar(v.T.Kind) {
			body = append(body, &PrintStmt{Arg: v})
		}
	}
	if len(body) == 0 {
		body = append(body, &PrintStmt{Arg: &IntLit{V: "0"}})
	}
	imports := make([]string, 0, len(st.imps))
	for k := range st.imps {
		imports = append(imports, k)
	}
	return &Program{Imports: imports, Body: body, Cats: cats, Fams: st.fams}
}

func isScalar(k Kind) bool { return k == KInt || k == KFloat || k == KBool || k == KString }

func appendFlat(body []Stmt, s Stmt) []Stmt {
	if g, ok := s.(*stmtGroup); ok {
		for _, inner := range g.ss {
			body = appendFlat(body, inner)
		}
		return body
	}
	return append(body, s)
}

// stmtGroup lets a generator helper emit several statements while genStmt returns
// a single Stmt; appendFlat flattens it so it never reaches Print/reductions,
// which only handle the three concrete Stmt types.
type stmtGroup struct{ ss []Stmt }

func (*stmtGroup) isStmt() {}

func blockStmts(ss ...Stmt) Stmt { return &stmtGroup{ss} }

func (st *genState) newVar(t Type) *Var {
	v := &Var{Name: "v" + strconv.Itoa(st.next), T: t}
	st.next++
	st.vars = append(st.vars, v)
	return v
}

// familyWitnesses emits one witness per admitted FAMILY, unconditionally and with
// FIXED SAFE arguments (no RNG draw, no abort-inducing input), so every family is
// exercised in every program and its value provably reaches stdout (SC-009 + FR-003
// witness rule) -- a witness that aborted mid-prelude would leave later witnesses
// unprinted while the IR tag still claimed coverage, so witnesses must never abort.
// It discovers the family set from admittedBuiltins (a stable slice) and dispatches
// to witnessForFamily, which names a specific admitted builtin of that family in a
// hand-verified safe shape (including composite-arg families like dict/array, which
// the generic scalar path cannot build). TestGeneratedProgramsAllCompile and the
// Task 10 family-witness test enforce completeness and correctness.
func (st *genState) familyWitnesses() []Stmt {
	var out []Stmt
	seen := map[string]bool{}
	for _, b := range admittedBuiltins {
		if seen[b.Family] {
			continue
		}
		seen[b.Family] = true
		st.fams.add(b.Family)
		out = append(out, st.witnessForFamily(b.Family, b))
	}
	return out
}

// categoryWitnesses emits one printed witness per pure-core AND payload category,
// unconditionally and deterministically, so every category is witnessed in every
// program with its value reaching stdout (SC-009 + FR-003 witness rule). Structural
// categories (control flow, array, dict) are witnessed by a minimal construct whose
// result is printed; value categories by a printed literal/expression exercising
// them. Each helper sets its own category tag via st.cats. NOTE: CatArithEdge uses
// INT_MAX (a benign 18-digit edge), never the INT_MIN-in-$(( )) construct, so the
// clean baseline (SC-005) is never perturbed by the coverage prelude; INT_MIN is
// emitted only by the payload/random walk that drives the done-signal.
func (st *genState) categoryWitnesses() []Stmt {
	var out []Stmt
	for _, c := range append(append([]Category{}, pureCoreCategories...), payloadCategories...) {
		out = append(out, st.witnessForCategory(c))
		st.cats.add(c)
	}
	return out
}

// witnessForCategory returns one printed statement whose value exercises category c.
// The exact shapes are confirmed to type-check and print; TestGeneratedProgramsAllCompile
// and TestRunSeededAccumulatesAllCoverage enforce completeness and correctness.
func (st *genState) witnessForCategory(c Category) Stmt {
	switch c {
	case CatIntArith:
		return &PrintStmt{Arg: &Binary{Op: "+", L: &IntLit{V: "1"}, R: &IntLit{V: "1"}, T: Type{Kind: KInt}}}
	case CatFloatArith:
		return &PrintStmt{Arg: &Binary{Op: "+", L: &FloatLit{V: "1.5"}, R: &FloatLit{V: "2.5"}, T: Type{Kind: KFloat}}}
	case CatBool:
		return &PrintStmt{Arg: &BoolLit{V: true}}
	case CatString:
		return &PrintStmt{Arg: &StringLit{V: "s"}}
	case CatArray:
		a := st.newVar(Type{Kind: KArray, Elem: &Type{Kind: KInt}})
		return blockStmts(
			&LetStmt{Name: a.Name, T: a.T, Init: &ArrayLit{Elems: []Expr{&IntLit{V: "1"}, &IntLit{V: "2"}}, Elem: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Call{Builtin: "length", Args: []Expr{a}, T: Type{Kind: KInt}}},
		)
	case CatDict:
		st.imps["dict"] = true
		d := st.newVar(Type{Kind: KDict, Elem: &Type{Kind: KInt}})
		return blockStmts(
			&LetStmt{Name: d.Name, T: d.T, Init: &DictLit{Keys: []string{"k"}, Vals: []Expr{&IntLit{V: "1"}}, Val: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Call{Builtin: "unwrap_or", Args: []Expr{&Call{Builtin: "dict.get", Args: []Expr{d, &StringLit{V: "k"}}, T: Type{Kind: KInt}}, &IntLit{V: "0"}}, T: Type{Kind: KInt}}},
		)
	case CatControlFlow:
		return blockStmts(&IfStmt{
			Cond: &BoolLit{V: true},
			Then: []Stmt{&PrintStmt{Arg: &IntLit{V: "1"}}},
			Else: []Stmt{&PrintStmt{Arg: &IntLit{V: "0"}}},
		})
	case CatArithEdge:
		return &PrintStmt{Arg: &IntLit{V: "9223372036854775807"}} // INT_MAX: benign edge, no $(( ))
	default:
		// payload categories: a printed string literal carrying the category's bytes.
		return &PrintStmt{Arg: &StringLit{V: payloadCategorySample(c)}}
	}
}

// payloadCategorySample returns a string containing bytes that exercise payload
// category c (matching genStringValue's category-bearing fragments). The value is a
// plain wisp string literal; Print/escapeWispString renders it safely and the shells
// reproduce it byte-for-byte.
func payloadCategorySample(c Category) string {
	switch c {
	case CatCmdSubst:
		return "$(echo x)"
	case CatNewline:
		return "a\nb"
	case CatFormatPct:
		return "%s%d"
	case CatBackslash:
		return "a\\b"
	case CatLeadDash:
		return "-n"
	case CatGlob:
		return "*.?["
	default:
		return "x"
	}
}

// witnessForFamily returns a statement that exercises family fam via a specific
// admitted builtin in a hand-verified SAFE shape whose scalar value reaches stdout
// (printed directly or bound and printed by the trailing scaffold). Fixed literals
// only -- no RNG, no INT_MIN, no abort. Composite families (array/dict) build their
// operand inline via blockStmts. The named builtins (to_string/length/is_some/Some/
// math.abs/string.upper/array.contains/dict.get/unwrap_or) are all in
// admittedBuiltins (see builtins.go), so TestGeneratorCallsOnlyAdmitted passes.
//
// Any admitted family WITHOUT an explicit case here -- parse (parse_int/float/bool
// are Optional scalars), debug (int->string), and regex (matches/replace are scalar;
// find is Optional; find_all is string[]) -- falls to the default arm, which sets
// b.Import and calls scalarBuiltinLet: it synthesizes safe scalar args, wraps a
// handle (Optional) return in unwrap_or, and binds+length-prints a non-scalar (array)
// return, so the witnessed value still reaches stdout. Two requirements for the
// default path to work: (1) the family's admitted representative `b` carries its
// `Import` (e.g. `Import:"regex"`) so the emitted program imports the namespace, and
// (2) every param is scalar (safeScalarArg panics on a composite param) -- true for
// parse/debug/regex. b is the discovered representative (used only by the default).
// The Task 10 family-witness test fails until every admitted family is witnessed, so
// a family needing more than the default path forces an explicit case.
func (st *genState) witnessForFamily(fam string, b builtinSig) Stmt {
	switch fam {
	case "convert":
		return &PrintStmt{Arg: &Call{Builtin: "to_string", Args: []Expr{&IntLit{V: "5"}}, T: Type{Kind: KString}}}
	case "collection":
		a := st.newVar(Type{Kind: KArray, Elem: &Type{Kind: KInt}})
		return blockStmts(
			&LetStmt{Name: a.Name, T: a.T, Init: &ArrayLit{Elems: []Expr{&IntLit{V: "1"}, &IntLit{V: "2"}}, Elem: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Call{Builtin: "length", Args: []Expr{a}, T: Type{Kind: KInt}}},
		)
	case "option":
		// Witness option via is_some applied to an admitted Optional producer
		// (parse_int) rather than the Some(...) reserved constructor, which is NOT a
		// catalog/admitted builtin -- keeping TestGeneratorCallsOnlyAdmitted's
		// structural proof honest (every emitted Call names an admitted builtin).
		st.fams.add("parse")
		return &PrintStmt{Arg: &Call{Builtin: "is_some", Args: []Expr{&Call{Builtin: "parse_int", Args: []Expr{&StringLit{V: "42"}}, T: Type{Kind: KInt}}}, T: Type{Kind: KBool}}}
	case "math":
		st.imps["math"] = true
		return &PrintStmt{Arg: &Call{Builtin: "math.abs", Args: []Expr{&IntLit{V: "5"}}, T: Type{Kind: KInt}}}
	case "string":
		st.imps["string"] = true
		return &PrintStmt{Arg: &Call{Builtin: "string.upper", Args: []Expr{&StringLit{V: "x"}}, T: Type{Kind: KString}}}
	case "array":
		st.imps["array"] = true
		a := st.newVar(Type{Kind: KArray, Elem: &Type{Kind: KInt}})
		return blockStmts(
			&LetStmt{Name: a.Name, T: a.T, Init: &ArrayLit{Elems: []Expr{&IntLit{V: "1"}, &IntLit{V: "2"}}, Elem: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Call{Builtin: "array.contains", Args: []Expr{a, &IntLit{V: "1"}}, T: Type{Kind: KBool}}},
		)
	case "dict":
		st.imps["dict"] = true
		d := st.newVar(Type{Kind: KDict, Elem: &Type{Kind: KInt}})
		return blockStmts(
			&LetStmt{Name: d.Name, T: d.T, Init: &DictLit{Keys: []string{"k"}, Vals: []Expr{&IntLit{V: "1"}}, Val: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Call{Builtin: "unwrap_or", Args: []Expr{&Call{Builtin: "dict.get", Args: []Expr{d, &StringLit{V: "k"}}, T: Type{Kind: KInt}}, &IntLit{V: "0"}}, T: Type{Kind: KInt}}},
		)
	default:
		if b.Import != "" {
			st.imps[b.Import] = true
		}
		return st.scalarBuiltinLet(b)
	}
}

// scalarBuiltinLet binds a printed scalar to a call of b with synthesized scalar
// args (used for the default-arm families parse/debug/regex). Handle returns are
// wrapped in unwrap_or so the value reaches stdout instead of staying an opaque
// Optional. A non-scalar return (e.g. string[]) is bound and length-printed. The
// bound scalar variable is printed by Generate's trailing value-print scaffold.
func (st *genState) scalarBuiltinLet(b builtinSig) Stmt {
	args := make([]Expr, len(b.Params))
	for i, p := range b.Params {
		args[i] = st.safeScalarArg(b.Name, p)
	}
	ret := b.Ret
	call := Expr(&Call{Builtin: b.Name, Args: args, T: ret})
	if b.RetHandle {
		st.fams.add("option")
		call = &Call{Builtin: "unwrap_or", Args: []Expr{call, leafOfType(ret)}, T: ret}
	}
	if !isScalar(ret.Kind) {
		// non-scalar return (e.g. array): bind it, then print length.
		av := st.newVar(ret)
		lv := st.newVar(Type{Kind: KInt})
		st.fams.add("collection")
		return blockStmts(
			&LetStmt{Name: av.Name, T: ret, Init: call},
			&LetStmt{Name: lv.Name, T: Type{Kind: KInt}, Init: &Call{Builtin: "length", Args: []Expr{av}, T: Type{Kind: KInt}}},
		)
	}
	v := st.newVar(ret)
	return &LetStmt{Name: v.Name, T: ret, Init: call}
}

// safeScalarArg returns a FIXED, safe value of type p for a builtin argument. It is
// deliberately conservative: builtin arguments never carry adversarial edge values
// (that would only make abort-capable builtins like abs/gcd/lcm/to_int/repeat abort
// -- uniformly across shells, so no divergence, just wasted programs and, in the
// witness prelude, unprinted trailing witnesses). Adversarial payloads and INT_MIN
// flow instead through SOURCE-LEVEL arithmetic and string let-bindings (genExpr /
// genIntValue / genStringValue), which is where cross-shell byte bugs actually live.
// Conversions get inputs valid for their target so the converted value reaches
// stdout instead of aborting.
func (st *genState) safeScalarArg(name string, p Type) Expr {
	switch p.Kind {
	case KString:
		switch name {
		case "to_int", "parse_int":
			return &StringLit{V: "42"}
		case "to_float", "parse_float":
			return &StringLit{V: "4.5"}
		case "to_bool", "parse_bool":
			return &StringLit{V: "true"}
		}
		return &StringLit{V: "x"}
	case KInt:
		return &IntLit{V: "3"} // small, safe: never INT_MIN/huge (no abort in abs/gcd/repeat)
	case KFloat:
		return &FloatLit{V: "1.5"}
	case KBool:
		return &BoolLit{V: true}
	}
	// safeScalarArg is SCALAR-ONLY by contract. It is never called with a composite
	// param today: intBuiltinCall gates on canSynthParams (all-scalar) and every
	// witnessForFamily case builds composite args inline. genExpr has no KArray/KDict
	// case, so a composite param here would silently yield an int (ill-typed) -- panic
	// instead so a future composite-param caller fails loudly at generation, not as a
	// confusing downstream compile error.
	panic(fmt.Sprintf("safeScalarArg: non-scalar param kind %v for builtin %q (build composite args in witnessForFamily)", p.Kind, name))
}

func (st *genState) genStmt() Stmt {
	switch st.r.Intn(6) {
	case 0:
		return st.genIf()
	case 1:
		return st.genArrayStmt()
	case 2:
		return st.genDictStmt()
	default:
		t := st.randScalarType()
		// Generate the initializer BEFORE registering the new var, so pickVar in the
		// initializer cannot select the very variable being defined (a self-reference
		// -- `let v8: int = (v8 + ...)` -- is undeclared-name at that point).
		init := st.genExpr(t, 0)
		return &LetStmt{Name: st.newVar(t).Name, T: t, Init: init}
	}
}

func (st *genState) genIf() Stmt {
	st.cats.add(CatControlFlow)
	then := []Stmt{&PrintStmt{Arg: &IntLit{V: "1"}}}
	var els []Stmt
	if st.r.Bool() {
		els = []Stmt{&PrintStmt{Arg: &IntLit{V: "2"}}}
	}
	return &IfStmt{Cond: st.genExpr(Type{Kind: KBool}, 0), Then: then, Else: els}
}

func (st *genState) genArrayStmt() Stmt {
	st.cats.add(CatArray)
	st.fams.add("collection")
	elemT := Type{Kind: KInt}
	n := 1 + st.r.Intn(4)
	elems := make([]Expr, n)
	for i := range elems {
		elems[i] = st.intLeaf()
	}
	av := st.newVar(Type{Kind: KArray, Elem: &elemT})
	lv := st.newVar(Type{Kind: KInt})
	return blockStmts(
		&LetStmt{Name: av.Name, T: av.T, Init: &ArrayLit{Elems: elems, Elem: elemT}},
		&LetStmt{Name: lv.Name, T: Type{Kind: KInt}, Init: &Call{Builtin: "length", Args: []Expr{av}, T: Type{Kind: KInt}}},
	)
}

func (st *genState) genDictStmt() Stmt {
	st.cats.add(CatDict)
	st.imps["dict"] = true
	st.fams.add("dict")
	st.fams.add("option") // unwrap_or
	valT := Type{Kind: KInt}
	n := 1 + st.r.Intn(3)
	keys := make([]string, n)
	vals := make([]Expr, n)
	for i := range keys {
		keys[i] = "k" + strconv.Itoa(i)
		vals[i] = st.intLeaf()
	}
	dv := st.newVar(Type{Kind: KDict, Elem: &valT})
	gv := st.newVar(Type{Kind: KInt})
	get := &Call{Builtin: "dict.get", Args: []Expr{dv, &StringLit{V: keys[0]}}, T: Type{Kind: KInt}}
	unwrapped := &Call{Builtin: "unwrap_or", Args: []Expr{get, &IntLit{V: "0"}}, T: Type{Kind: KInt}}
	return blockStmts(
		&LetStmt{Name: dv.Name, T: dv.T, Init: &DictLit{Keys: keys, Vals: vals, Val: valT}},
		&LetStmt{Name: gv.Name, T: Type{Kind: KInt}, Init: unwrapped},
	)
}

func (st *genState) randScalarType() Type {
	switch st.r.Intn(4) {
	case 0:
		return Type{Kind: KInt}
	case 1:
		return Type{Kind: KFloat}
	case 2:
		return Type{Kind: KBool}
	default:
		return Type{Kind: KString}
	}
}

func (st *genState) genExpr(t Type, depth int) Expr {
	atMax := depth >= st.cfg.MaxDepth
	switch t.Kind {
	case KInt:
		st.cats.add(CatIntArith)
		if atMax || st.r.Intn(3) == 0 {
			return st.intLeaf()
		}
		return &Binary{Op: []string{"+", "-", "*"}[st.r.Intn(3)], L: st.genExpr(t, depth+1), R: st.genExpr(t, depth+1), T: t}
	case KFloat:
		st.cats.add(CatFloatArith)
		if atMax || st.r.Intn(2) == 0 {
			return genFloatValue(st.r)
		}
		return &Binary{Op: []string{"+", "-", "*"}[st.r.Intn(3)], L: st.genExpr(t, depth+1), R: st.genExpr(t, depth+1), T: t}
	case KBool:
		st.cats.add(CatBool)
		l := st.genExpr(Type{Kind: KInt}, depth+1)
		r := st.genExpr(Type{Kind: KInt}, depth+1)
		return &Binary{Op: []string{"==", "!=", "<", ">", "<=", ">="}[st.r.Intn(6)], L: l, R: r, T: Type{Kind: KBool}}
	case KString:
		return genStringValue(st.r, st.cats)
	default:
		return st.intLeaf()
	}
}

func (st *genState) intLeaf() Expr {
	switch st.r.Intn(4) {
	case 0:
		if v := st.pickVar(KInt); v != nil {
			return v
		}
	case 1:
		return st.intBuiltinCall()
	}
	return genIntValue(st.r, st.cats)
}

// intBuiltinCall calls an admitted int-returning builtin with scalar params only
// (so `length`, needing int[], is excluded here; arrays are witnessed elsewhere).
func (st *genState) intBuiltinCall() Expr {
	var cands []builtinSig
	for _, b := range admittedBuiltins {
		// !b.Special excludes special-call-shape builtins (Some/None/unwrap/Ok/Err/...)
		// whose nil Params + zero-value Ret would otherwise be misread as a zero-arg
		// int builtin and emitted as an ill-typed bare call. Only genuinely
		// generically-callable int builtins remain (to_int, math.abs/min/max,
		// math.int_min/int_max).
		if b.Ret.Kind == KInt && !b.RetHandle && !b.Special && st.canSynthParams(b.Params) {
			cands = append(cands, b)
		}
	}
	if len(cands) == 0 {
		return genIntValue(st.r, st.cats)
	}
	b := cands[st.r.Intn(len(cands))]
	if b.Import != "" {
		st.imps[b.Import] = true
	}
	st.fams.add(b.Family)
	args := make([]Expr, len(b.Params))
	for i, p := range b.Params {
		// Route through safeScalarArg so conversion builtins (to_int/to_float/to_bool)
		// receive a valid literal and their result reaches stdout instead of aborting
		// mid-program (which would truncate later witness prints -- SC-009).
		args[i] = st.safeScalarArg(b.Name, p)
	}
	if b.Name == "math.int_min" || b.Name == "math.int_max" {
		st.cats.add(CatArithEdge)
	}
	return &Call{Builtin: b.Name, Args: args, T: Type{Kind: KInt}}
}

func (st *genState) canSynthParams(ps []Type) bool {
	for _, p := range ps {
		if !isScalar(p.Kind) {
			return false
		}
	}
	return true
}

func (st *genState) pickVar(k Kind) *Var {
	var cands []*Var
	for _, v := range st.vars {
		if v.T.Kind == k {
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	return cands[st.r.Intn(len(cands))]
}
