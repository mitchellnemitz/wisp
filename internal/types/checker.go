package types

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// Check type-checks and resolves a single-module program (no imports/includes).
// It wraps the module-aware CheckLinked so all existing single-file callers and
// tests are unchanged. It always returns a non-nil *Info; the program is accepted
// iff len(Info.Errors) == 0. Warnings never gate.
func Check(prog *ast.Program) *Info {
	return CheckLinked(&module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: prog, Namespaces: map[string]int{}},
	}})
}

type checker struct {
	info *Info

	// modCtx[i] holds module i's symbol tables; cur is the module being checked.
	modCtx []*moduleCtx
	cur    *moduleCtx

	// per-function mangle counter for block-scoped variable names.
	varCounter int
	// current function being checked (for collecting Decls and return type).
	curFunc *FuncInfo
	curRet  Type
	// loopDepth > 0 means break/continue are legal.
	loopDepth int
	// tryDepth > 0 means we are inside a try/catch/finally body, where
	// return/break/continue are compile errors in M5 (spec 4.1).
	tryDepth int
	// loopTryDepth[i] records the tryDepth in effect when the i-th enclosing loop
	// was entered. A break/continue targets the innermost loop; if that loop was
	// opened OUTSIDE the current try (loopTryDepth[last] < tryDepth), the transfer
	// would escape the try body and is a compile error in M5. A loop fully inside
	// the try (equal depth) keeps its own break/continue legal.
	loopTryDepth []int

	// constResolver is an optional hook wired by Task 3 (checker collect-fold
	// pass). When non-nil, checkConstExpr calls it before rejecting an Ident as
	// "not a constant expression". The callback receives the identifier name and
	// returns the folded value, its type, and true if the name is a known const;
	// or (nil, Invalid, false) if the name is not a const.
	constResolver func(name string) (interface{}, Type, bool)

	// foldAllowsQualified permits foldConst's FieldAccess arm to resolve a
	// cross-module `ns.NAME` const. It is true ONLY while folding a
	// default-argument expression (set around the checkConstExpr calls in
	// checkParams), and false everywhere else, so a const INITIALIZER stays
	// file-local (a ns.NAME there is a compile error, AC6) while a default arg
	// accepts it (AC3/R10).
	foldAllowsQualified bool

	// scope stack: scopes[0] is the function's parameter scope.
	scopes []scope

	// typeParams is the set of type-parameter names in scope for the function
	// currently being checked (nil when checking a non-generic function or
	// outside any function). resolveType maps a name in this set to its type
	// variable encoding "$name" instead of treating it as an unknown struct.
	typeParams map[string]bool

	// typeParamBounds maps a type-parameter name in scope to its bound
	// ("comparable"); absent = unbounded. Same lifetime as typeParams.
	typeParamBounds map[string]string

	// seen tracks (Pos,Msg) pairs already recorded, so a diagnostic emitted
	// twice (e.g. probe-then-fallthrough builtins re-checking the same arg)
	// is reported once. Lazily initialized in errf.
	seen map[string]bool
}

// isComparableTypeVar reports whether t is a type variable carrying the
// comparable bound.
func (c *checker) isComparableTypeVar(t Type) bool {
	return isTypeVar(t) && c.typeParamBounds[typeVarName(t)] == "comparable"
}

// isNumericTypeVar reports whether t is a type variable carrying the
// numeric bound.
func (c *checker) isNumericTypeVar(t Type) bool {
	return isTypeVar(t) && c.typeParamBounds[typeVarName(t)] == "numeric"
}

// IsTypeVar is the exported form of isTypeVar; used by codegen.
func IsTypeVar(t Type) bool { return isTypeVar(t) }

// typeVarType is the type-variable encoding for a type parameter named name.
// The "$" prefix is illegal in source identifiers and in every composite type
// encoding, so a type variable can never collide with a user-writable type.
func typeVarType(name string) Type { return Type("$" + name) }

// isTypeVar reports whether t is a type-variable encoding ("$name").
func isTypeVar(t Type) bool { return len(t) > 0 && t[0] == '$' }

// scope is one lexical block's name table.
type scope map[string]*Var

func (c *checker) errf(pos token.Position, format string, args ...any) {
	msg := c.diagMsg(format, args...)
	key := pos.String() + "\x00" + msg
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.info.Errors = append(c.info.Errors, Diagnostic{Pos: pos, Msg: msg})
}

func (c *checker) warnf(pos token.Position, format string, args ...any) {
	c.info.Warnings = append(c.info.Warnings, Diagnostic{Pos: pos, Msg: c.diagMsg(format, args...)})
}

// diagMsg formats a diagnostic, rendering every Type argument through disp (so a
// struct's internal token Name@modid shows as the readable Name, never with @),
// and appends the import/include chain trailer when the current module is not the
// root (acceptance 5). Single-module struct types are modid 0, so disp strips
// `@0` and the message is identical to the pre-M8 bare-name form.
func (c *checker) diagMsg(format string, args ...any) string {
	for i, a := range args {
		if t, ok := a.(Type); ok {
			args[i] = disp(t)
		}
	}
	msg := fmt.Sprintf(format, args...)
	if c.cur != nil && len(c.cur.chain) > 0 {
		msg += chainTrailer(c.cur.chain)
	}
	return msg
}

// collectStructs records each struct's name and field table for module ctx,
// rejecting reserved or duplicate struct names and duplicate field names. Each
// struct is stored in the global info.Structs under its INTERNAL token
// (Name@modid) and in ctx.structs by its source name; an exported struct is
// recorded in ctx.exported. Field TYPES are resolved in a second pass
// (checkStructFields) so a field may reference another (possibly cross-module)
// struct. The caller sets c.cur = ctx first.
func (c *checker) collectStructs(ctx *moduleCtx) {
	for _, sd := range ctx.prog.Structs {
		if isReservedIdent(sd.Name) {
			c.errf(sd.NamePos, "%q uses the reserved \"__\" namespace and cannot be a struct name", sd.Name)
		}
		if isReservedName(sd.Name) {
			c.errf(sd.NamePos, "%q is a reserved builtin or constant name and cannot be a struct name", sd.Name)
		}
		if isPrimitiveTypeName(sd.Name) {
			c.errf(sd.NamePos, "%q is a built-in type name and cannot be a struct name", sd.Name)
		}
		if _, dup := ctx.structs[sd.Name]; dup {
			c.errf(sd.NamePos, "struct %q is declared more than once", sd.Name)
			continue
		}
		si := &StructInfo{Decl: sd, Name: sd.Name, ID: ctx.id, TypeParams: sd.TypeParams, byName: map[string]Type{}}
		seen := map[string]bool{}
		for _, f := range sd.Fields {
			if isReservedIdent(f.Name) {
				c.errf(f.NamePos, "%q uses the reserved \"__\" namespace and cannot be a field name", f.Name)
			}
			if seen[f.Name] {
				c.errf(f.NamePos, "field %q is declared more than once in struct %q", f.Name, sd.Name)
				continue
			}
			seen[f.Name] = true
			// type resolved later; placeholder Invalid for now.
			si.Fields = append(si.Fields, StructFieldInfo{Name: f.Name})
		}
		c.info.Structs[string(internalStructName(sd.Name, ctx.id))] = si
		ctx.structs[sd.Name] = si
		if sd.Exported {
			ctx.exported[sd.Name] = true
		}
	}
}

// checkStructFields resolves and validates each struct field's type for module
// ctx, now that every module's struct names are known (so a field may reference a
// qualified struct in another module). The caller sets c.cur = ctx first.
func (c *checker) checkStructFields(ctx *moduleCtx) {
	for _, sd := range ctx.prog.Structs {
		si := ctx.structs[sd.Name]
		if si == nil {
			continue // duplicate; already reported
		}
		// For generic structs, temporarily expose the struct's type-parameter names
		// as type-vars so field types like T resolve to $T rather than failing.
		savedTypeParams := c.typeParams
		if len(sd.TypeParams) > 0 {
			c.typeParams = map[string]bool{}
			for _, tp := range sd.TypeParams {
				c.typeParams[tp] = true
			}
		}
		fi := 0
		fieldSeen := map[string]bool{}
		for _, f := range sd.Fields {
			if fieldSeen[f.Name] {
				continue
			}
			fieldSeen[f.Name] = true
			ft := c.resolveType(f.Type, f.NamePos)
			si.byName[f.Name] = ft
			if fi < len(si.Fields) {
				si.Fields[fi].Type = ft
			}
			fi++
		}
		if len(sd.TypeParams) > 0 {
			c.typeParams = savedTypeParams
		}
	}
}

// collectFuncs records every function signature for module ctx and rejects
// reserved or duplicate function names. Exported functions are recorded in
// ctx.exported. It does not check bodies. The caller sets c.cur = ctx first.
func (c *checker) collectFuncs(ctx *moduleCtx) {
	for _, fn := range ctx.prog.Funcs {
		if isReservedIdent(fn.Name) {
			c.errf(fn.KwPos, "%q uses the reserved \"__\" namespace and cannot be a function name", fn.Name)
		}
		if isReservedName(fn.Name) {
			c.errf(fn.KwPos, "%q is a reserved builtin or constant name and cannot be redefined", fn.Name)
		}
		if _, dup := ctx.funcs[fn.Name]; dup {
			c.errf(fn.KwPos, "function %q is declared more than once", fn.Name)
			continue
		}
		for _, tp := range fn.TypeParams {
			if isReservedIdent(tp) {
				c.errf(fn.KwPos, "type parameter %q uses the reserved \"__\" namespace", tp)
			}
			if _, ok := ctx.structs[tp]; ok {
				c.errf(fn.KwPos, "type parameter %q collides with struct %q in scope", tp, tp)
			}
		}
		ctx.funcs[fn.Name] = fn
		if fn.Exported {
			ctx.exported[fn.Name] = true
		}
	}
	// Reject a user function whose name collides with one the monomorphizer would
	// generate for a numeric-bounded generic. `add[T: numeric]` emits instances
	// `add__int`/`add__float`; a user function literally named `add__int` mangles
	// to the same shell function and would silently win. Both source names are
	// distinct (so the duplicate check above does not catch it), but the generated
	// shell names collide.
	for _, fn := range ctx.prog.Funcs {
		for _, suf := range monoNameSuffixes(fn) {
			collide := fn.Name + suf
			if other, ok := ctx.funcs[collide]; ok && other != fn {
				c.errf(other.KwPos, "function %q collides with a monomorphization of the numeric generic %q; rename one of them", collide, fn.Name)
			}
		}
	}
}

// monoNameSuffixes returns the name suffixes the monomorphizer appends for a
// numeric-bounded generic function (one "__int"/"__float" per numeric type
// parameter, cartesian over multiple). Returns nil for a function with no
// numeric-bounded type parameters (which is not monomorphized by suffix).
func monoNameSuffixes(fn *ast.FuncDecl) []string {
	var numericCount int
	for _, tp := range fn.TypeParams {
		if fn.TypeParamBounds[tp] == "numeric" {
			numericCount++
		}
	}
	if numericCount == 0 {
		return nil
	}
	sufs := []string{""}
	for i := 0; i < numericCount; i++ {
		next := make([]string, 0, len(sufs)*2)
		for _, s := range sufs {
			next = append(next, s+"__"+string(Int), s+"__"+string(Float))
		}
		sufs = next
	}
	return sufs
}

// isTestFile reports whether file names a `*_test.wisp` test file. The `test`
// construct, and the no-`fn main` exemption, apply only to such files.
func isTestFile(file string) bool {
	return strings.HasSuffix(file, "_test.wisp")
}

// validateRootMain enforces: exactly one `fn main() -> int` (or a single
// `args: string[]` parameter) in the ROOT program. In test-file mode (the root
// is a `*_test.wisp`) a missing `fn main` is NOT an error -- a test file is not a
// program (R2/AC15) -- but a `main` that IS present is still validated.
func (c *checker) validateRootMain(prog *ast.Program, testMode bool) {
	var mains []*ast.FuncDecl
	for _, fn := range prog.Funcs {
		if fn.Name == "main" {
			mains = append(mains, fn)
		}
	}
	switch len(mains) {
	case 0:
		if testMode {
			return // a *_test.wisp file legitimately has no `fn main`
		}
		c.errf(token.Position{File: prog.File, Line: 1, Col: 1}, "no main function: a program must define exactly one `fn main() -> int`")
		return
	case 1:
		// validated below
	default:
		for _, m := range mains[1:] {
			c.errf(m.KwPos, "main is declared more than once")
		}
	}
	m := mains[0]
	ok := true
	// main may take no parameters, or a single `args: string[]` parameter (spec
	// 4.5). Any other parameter shape is an error.
	mainArgs := false
	switch len(m.Params) {
	case 0:
		// fn main() -> int
	case 1:
		p := m.Params[0]
		if p.Type != ast.ArrayType(ast.TypeString) {
			c.errf(p.NamePos, "main's parameter must be of type string[], got %s", ast.CanonicalType(p.Type))
			ok = false
		} else if p.Default != nil {
			c.errf(p.NamePos, "main's parameter may not have a default")
			ok = false
		} else {
			mainArgs = true
		}
	default:
		c.errf(m.KwPos, "main must take either no parameters or a single args: string[] parameter")
		ok = false
	}
	if m.RetType != ast.TypeInt {
		c.errf(m.KwPos, "main must return int, got %s", m.RetType)
		ok = false
	}
	if ok {
		c.info.Main = m
		c.info.MainArgs = mainArgs
	}
}

// --- function checking ---

func (c *checker) checkFunc(fn *ast.FuncDecl) {
	c.varCounter = 0
	c.loopDepth = 0
	c.tryDepth = 0
	c.loopTryDepth = nil
	c.typeParams = nil
	c.typeParamBounds = nil
	if len(fn.TypeParams) > 0 {
		c.typeParams = map[string]bool{}
		for _, p := range fn.TypeParams {
			c.typeParams[p] = true
		}
		c.typeParamBounds = fn.TypeParamBounds
	}
	c.curRet = c.resolveType(fn.RetType, fn.KwPos)
	fi := &FuncInfo{Decl: fn, Mangled: mangleFunc(c.cur.id, fn.Name)}
	c.info.Funcs[fn] = fi
	c.curFunc = fi

	// parameter scope
	c.scopes = []scope{{}}
	c.checkParams(fn)

	c.checkBlock(fn.Body)

	// all-paths-return for non-void functions
	if c.curRet != Void && !blockReturns(fn.Body, c.info) {
		c.errf(fn.KwPos, "function %q must return %s on every path", fn.Name, c.curRet)
	}

	c.popScopeWarnUnused()
	c.curFunc = nil
	c.typeParams = nil
	c.typeParamBounds = nil
}

func (c *checker) checkParams(fn *ast.FuncDecl) {
	seen := map[string]bool{}
	sawDefault := false
	for i := range fn.Params {
		p := &fn.Params[i]
		if p.Name == "_" {
			// Blank parameter: type-check the annotation and any default, but create
			// no Var, no fi.Decls entry, and skip the duplicate-name check (multiple
			// `_` params are legal). The positional slot is consumed at call time.
			pt := c.resolveType(p.Type, p.NamePos)
			if p.Default != nil {
				sawDefault = true
				c.foldAllowsQualified = true
				ct := c.checkConstExpr(p.Default)
				c.foldAllowsQualified = false
				if ct != Invalid && ct != pt {
					c.errf(p.Default.Pos(), "default for parameter %q has type %s, want %s", p.Name, ct, pt)
				}
			} else if sawDefault {
				c.errf(p.NamePos, "parameter %q has no default but follows a defaulted parameter; only trailing parameters may have defaults", p.Name)
			}
			continue
		}
		// reserved / builtin names
		if isReservedIdent(p.Name) {
			c.errf(p.NamePos, "%q uses the reserved \"__\" namespace and cannot be a parameter name", p.Name)
		} else if isReservedName(p.Name) {
			c.errf(p.NamePos, "%q is a reserved builtin or constant name and cannot be a parameter name", p.Name)
		}
		if seen[p.Name] {
			c.errf(p.NamePos, "parameter %q is declared more than once", p.Name)
		}
		seen[p.Name] = true
		if _, ok := c.cur.funcs[p.Name]; ok {
			c.errf(p.NamePos, "%q is a declared function and cannot be shadowed by a parameter", p.Name)
		}

		pt := c.resolveType(p.Type, p.NamePos)
		v := &Var{Name: p.Name, Mangled: c.mangleVar(), Type: pt, Pos: p.NamePos, IsParam: true, Used: true}
		c.scopes[0][p.Name] = v
		c.curFunc.Decls = append(c.curFunc.Decls, v)

		// default-argument validity (spec 10.3): const-expr of the param type,
		// trailing-only.
		if p.Default != nil {
			sawDefault = true
			c.foldAllowsQualified = true
			ct := c.checkConstExpr(p.Default)
			c.foldAllowsQualified = false
			if ct != Invalid && ct != pt {
				c.errf(p.Default.Pos(), "default for parameter %q has type %s, want %s", p.Name, ct, pt)
			}
		} else if sawDefault {
			c.errf(p.NamePos, "parameter %q has no default but follows a defaulted parameter; only trailing parameters may have defaults", p.Name)
		}
	}
}

// --- scopes ---

func (c *checker) pushScope() { c.scopes = append(c.scopes, scope{}) }

// popScopeWarnUnused pops the innermost scope, emitting an unused-local warning
// for any non-parameter variable that was never used.
func (c *checker) popScopeWarnUnused() {
	top := c.scopes[len(c.scopes)-1]
	for _, v := range top {
		if !v.IsParam && !v.Used {
			c.warnf(v.Pos, "unused variable %q", v.Name)
		}
	}
	c.scopes = c.scopes[:len(c.scopes)-1]
}

// lookup resolves name against the scope stack, innermost first.
func (c *checker) lookup(name string) *Var {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if v, ok := c.scopes[i][name]; ok {
			return v
		}
	}
	return nil
}

// declare adds a let binding to the innermost scope, enforcing no
// redeclaration and no shadowing (rule 11).
func (c *checker) declare(let *ast.LetStmt, t Type) *Var {
	if isReservedIdent(let.Name) {
		c.errf(let.KwPos, "%q uses the reserved \"__\" namespace and cannot be a variable name", let.Name)
	} else if isReservedName(let.Name) {
		c.errf(let.KwPos, "%q is a reserved builtin or constant name and cannot be a variable name", let.Name)
	}
	if prev := c.lookup(let.Name); prev != nil {
		c.errf(let.KwPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", let.Name, prev.Pos)
	}
	if _, ok := c.cur.funcs[let.Name]; ok {
		c.errf(let.KwPos, "%q is a declared function and cannot be shadowed by a variable", let.Name)
	}
	v := &Var{Name: let.Name, Mangled: c.mangleVar(), Type: t, Pos: let.KwPos}
	c.scopes[len(c.scopes)-1][let.Name] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
	c.info.Vars[let] = v
	return v
}

func (c *checker) mangleVar() string {
	c.varCounter++
	return fmt.Sprintf("__wisp_v_%d", c.varCounter)
}

// mangleFunc builds a function's shell name. The compiler-assigned modid (root =
// 0) makes the (modid, name) -> string map injective: a user name always starts
// with [A-Za-z_], never a digit, so two distinct pairs never collide (M8).
func mangleFunc(modid int, name string) string {
	return fmt.Sprintf("__wisp_f_m%d_%s", modid, name)
}

// funcRefType builds the function-reference Type for a declared function: its
// FULL declared parameter list (defaults do NOT participate, spec 2.2) and its
// return type. The encoding mirrors the array/dict composites so type equality
// is plain ==.
func (c *checker) funcRefType(fn *ast.FuncDecl) Type {
	params := make([]Type, len(fn.Params))
	for i := range fn.Params {
		params[i] = c.resolveType(fn.Params[i].Type, fn.Params[i].NamePos)
	}
	ret := c.resolveType(fn.RetType, fn.KwPos)
	return funcType(params, ret)
}

// funcType encodes parameter types and return type into the structural
// "fn(...)->R" Type string.
func funcType(params []Type, ret Type) Type {
	s := "fn("
	for i, p := range params {
		if i > 0 {
			s += ","
		}
		s += string(p)
	}
	return Type(s + ")->" + string(ret))
}

// toType maps a primitive type-name annotation to its Type. It does NOT resolve
// composite (array/struct) annotations; callers in a checker context that may
// see a composite annotation must use c.resolveType instead. For a composite
// annotation toType returns Invalid.
func toType(t ast.TypeName) Type {
	switch t {
	case ast.TypeInt:
		return Int
	case ast.TypeFloat:
		return Float
	case ast.TypeBool:
		return Bool
	case ast.TypeString:
		return String
	case ast.TypeVoid:
		return Void
	default:
		return Invalid
	}
}

// isPrimitiveTypeName reports whether name is one of the built-in type keywords
// (so it cannot be a user struct name).
func isPrimitiveTypeName(name string) bool {
	switch ast.TypeName(name) {
	case ast.TypeInt, ast.TypeFloat, ast.TypeBool, ast.TypeString, ast.TypeVoid:
		return true
	}
	return false
}

// resolveType maps any type annotation -- primitive, array, or named struct --
// to its Type, validating that a named struct exists and that an array's
// element type resolves. An unknown named type is an error at pos and resolves
// to Invalid. The struct table must already be populated (collectStructs).
// noneNeedsContext reports the spec-5.4 error for a None used with no expected
// Optional[T]. It is the standard diagnostic for an unconcretizable None. One
// path reports a different primary error instead: `return None` in a void
// function reports "return with a value" and skips typing the None, so this is
// not literally the only message a stray None can produce.
func (c *checker) noneNeedsContext(pos token.Position) {
	c.errf(pos, "none requires an expected Optional type here; annotate the binding or use it in an Optional context")
}

// isNoneLiteral reports whether e is the bare `None` keyword node (an *ast.Ident
// named "None"). The blessed expected-type sites special-case it before any
// bottom-up check so it concretizes to the expected Optional[T] (and so it never
// reaches checkExpr, which would error it).
func isNoneLiteral(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "None"
}

func (c *checker) resolveType(t ast.TypeName, pos token.Position) Type {
	if c.typeParams[string(t)] {
		return typeVarType(string(t))
	}
	if p := toType(t); p != Invalid {
		return p
	}
	if t == ast.TypeName(ErrorType) {
		return ErrorType // built-in error handle type (M5)
	}
	if t == ast.TypeName(RunResult) {
		return RunResult // built-in RunResult handle type (R3)
	}
	if t == ast.TypeName(Process) {
		return Process
	}
	s := string(t)
	if strings.HasPrefix(s, "Optional[") && strings.HasSuffix(s, "]") {
		elem := c.resolveType(ast.TypeName(s[len("Optional["):len(s)-1]), pos)
		if elem == Invalid {
			return Invalid
		}
		if elem == Void {
			c.errf(pos, "optional element type cannot be void")
			return Invalid
		}
		return optionalType(elem)
	}
	if strings.HasPrefix(s, "Result[") && strings.HasSuffix(s, "]") {
		elem := c.resolveType(ast.TypeName(s[len("Result["):len(s)-1]), pos)
		if elem == Invalid {
			return Invalid
		}
		if elem == Void {
			c.errf(pos, "result success type cannot be void")
			return Invalid
		}
		return resultType(elem)
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		elem := c.resolveType(ast.TypeName(s[1:len(s)-1]), pos)
		if elem == Invalid {
			return Invalid
		}
		if elem == Void {
			c.errf(pos, "array element type cannot be void")
			return Invalid
		}
		return arrayType(elem)
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && strings.Contains(s, ":") {
		inner := s[1 : len(s)-1]
		i := strings.IndexByte(inner, ':')
		kAnn := ast.TypeName(inner[:i])
		vAnn := ast.TypeName(inner[i+1:])
		// Key type must be int or string (the only hashable keys, spec 4.4).
		if kAnn != ast.TypeInt && kAnn != ast.TypeString {
			c.errf(pos, "dict key type must be int or string, got %s", kAnn)
			c.resolveType(vAnn, pos) // surface value-type errors too
			return Invalid
		}
		kt := c.resolveType(kAnn, pos)
		vt := c.resolveType(vAnn, pos)
		if kt == Invalid || vt == Invalid {
			return Invalid
		}
		if vt == Void {
			c.errf(pos, "dict value type cannot be void")
			return Invalid
		}
		return dictType(kt, vt)
	}
	// Tuple type "(T1,T2,...,Tn)" -- starts with "(" and ends with ")"
	// and is not a funcref (which starts with "fn(").
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && !strings.HasPrefix(s, "fn(") {
		inner := s[1 : len(s)-1]
		rawElems := splitTopLevel(inner)
		// Enforce arity >= 2 (the parser already rejected 0 and 1 via parse errors;
		// this guard defends against hand-constructed TypeNames).
		if len(rawElems) < 2 {
			c.errf(pos, "tuple type requires at least two element types")
			return Invalid
		}
		elems := make([]Type, 0, len(rawElems))
		for _, raw := range rawElems {
			et := c.resolveType(ast.TypeName(raw), pos)
			if et == Invalid {
				return Invalid
			}
			if et == Void {
				c.errf(pos, "tuple element type cannot be void")
				return Invalid
			}
			elems = append(elems, et)
		}
		return tupleType(elems)
	}
	if strings.HasPrefix(s, "fn(") && strings.Contains(s, ")->") {
		return c.resolveFuncType(ast.TypeName(s), pos)
	}
	// Generic struct instantiation: "Box[int]", "Pair[int,string]". The bracket
	// index must be > 0 so bare arrays ("[int]", i=0) are not caught here.
	if i := strings.IndexByte(s, '['); i > 0 && strings.HasSuffix(s, "]") {
		if _, ok := c.cur.aliases[s[:i]]; ok {
			c.errf(pos, "type alias %q is not generic; aliases cannot take type arguments", s[:i])
			return Invalid
		}
		return c.resolveGenericStructInst(s[:i], s[i+1:len(s)-1], pos)
	}
	if _, ok := c.cur.aliases[s]; ok {
		return c.resolveAlias(c.cur, s)
	}
	return c.resolveNamedType(s, pos)
}

// resolveFuncType validates and re-encodes a function-reference annotation
// "fn(P1,...)->R" (M4): every parameter type resolves and is non-void (a void
// parameter is rejected, mirroring the array-element/dict-value void guards);
// the return type R resolves and MAY be void. Returns the canonical funcref
// Type, or Invalid on any error.
func (c *checker) resolveFuncType(t ast.TypeName, pos token.Position) Type {
	pAnns, rAnn := splitFuncTypeAnn(string(t))
	params := make([]Type, len(pAnns))
	bad := false
	for i, pa := range pAnns {
		pt := c.resolveType(ast.TypeName(pa), pos)
		if pt == Invalid {
			bad = true
		} else if pt == Void {
			c.errf(pos, "function parameter type cannot be void")
			bad = true
		}
		params[i] = pt
	}
	ret := c.resolveType(ast.TypeName(rAnn), pos) // void allowed for the return type
	if ret == Invalid {
		bad = true
	}
	if bad {
		return Invalid
	}
	return funcType(params, ret)
}

// splitFuncTypeAnn decomposes the annotation text "fn(P1,...)->R" into the
// parameter annotation strings and the return annotation string, matching the
// balanced-paren / top-level-comma split used for the resolved Type encoding.
func splitFuncTypeAnn(s string) ([]string, string) {
	params, ret := splitFuncType(Type(s))
	ps := make([]string, len(params))
	for i, p := range params {
		ps[i] = string(p)
	}
	return ps, string(ret)
}
