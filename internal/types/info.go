// Package types is the wisp type checker / resolver.
//
// Check walks an *ast.Program, resolves the type of every expression,
// enforces the strictness rules of spec section 7 (plus the reserved-name,
// switch-subject, constant-argument, default-argument, and main-signature
// rules), and produces an Info value that gives codegen everything it needs
// without re-deriving anything from the AST.
//
// # Result model
//
// Check always returns a non-nil *Info. A program is accepted iff
// len(Info.Errors) == 0. Warnings (spec rule 6/10) never gate: they are
// reported separately in Info.Warnings and a program with only warnings still
// compiles. Callers must therefore gate on Errors, not on Warnings.
//
// # Keying
//
// All ast expression nodes are pointer types, so the maps below are keyed by
// node pointer identity (ast.Expr / ast.Node interface values wrapping a
// pointer). Variable declarations are keyed by *ast.LetStmt and by the loop
// variable's *ast.LetStmt in a for-init; parameters are keyed by their *Var
// (looked up from the function via Funcs). Codegen walks the same AST and looks
// each node up here rather than recomputing types or names.
package types

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// Type is a resolved wisp type. Void appears only as a function/expression
// result type (a void call used as a statement); it is never the type of a
// value-producing expression operand.
type Type string

const (
	Int     Type = "int"
	Float   Type = "float"
	Bool    Type = "bool"
	String  Type = "string"
	Void    Type = "void"
	Invalid Type = "invalid" // assigned to ill-typed expressions so checking can continue
	// ErrorType is the built-in error handle type (M5): a reference type with one
	// field, message: string. Like array/dict/struct handles it is opaque (no
	// int/arith/compare); it is the type bound by catch and required by throw.
	ErrorType Type = "error"
	// RunResult is the built-in opaque handle type returned by run_full (R3):
	// three read-only fields stdout/stderr: string, code: int. Constructed only
	// by run_full; no compare, arithmetic, or string conversion.
	RunResult Type = "RunResult"
	// Process is the built-in opaque handle type returned by spawn (background
	// processes): one read-only field pid: int. Constructed only by spawn; no
	// compare, arithmetic, or string conversion. The wrapper pid, temp paths,
	// cached result, and state are internal (not language-visible).
	Process Type = "Process"
)

// Diagnostic is a compile-time message carrying a source position.
type Diagnostic struct {
	Pos token.Position
	Msg string
}

func (d Diagnostic) String() string { return d.Pos.String() + ": " + d.Msg }

// ConstEntry is one resolved top-level constant: its folded compile-time value
// (int64/bool/string, or FoldedFloat for a float const) and its resolved type.
// Stored in Info.ConstTable by the collect-fold pass (Task 3). Task 4 reads this
// table when resolving const references inside function bodies.
type ConstEntry struct {
	Value interface{} // int64, bool, string, or FoldedFloat (float const)
	Type  Type
}

// Var is a resolved variable declaration (a let binding, a for-init let, or a
// function parameter). Mangled is the unique shell variable name codegen emits;
// it lives in the reserved "__" namespace and is unique within the enclosing
// function, so two sibling-scope declarations of the same source name get
// distinct mangled names.
type Var struct {
	Name        string         // source name
	Mangled     string         // unique shell name, e.g. "__wisp_v_3"
	Type        Type           // declared type
	Pos         token.Position // declaration position
	IsParam     bool           // true for function parameters
	IsConst     bool           // true for const bindings (local or top-level)
	FoldedValue interface{}    // non-nil for IsConst Vars: the folded compile-time value (int64/bool/string, or FoldedFloat for a float const); codegen inlines this at use sites
	Immutable   bool           // true for final bindings (runtime-immutable local; NOT a const -- has a Mangled name and goes in Decls)
	Used        bool           // referenced after declaration (for the unused-local warning)
}

// CallKind distinguishes a builtin call from a user-function call.
type CallKind int

const (
	CallUser CallKind = iota
	CallBuiltin
	// CallIndirect is a call through a function reference (M4): the callee is an
	// expression of function type (a local funcref var, a param, a field, an array
	// element, a dict value, or another call's result). Codegen spills the callee
	// to a temp and invokes it as `"$ftmp" "$a" "$b"`.
	CallIndirect
)

// CallInfo is the resolved form of a call expression. Args is the full,
// defaults-filled argument list in parameter order: omitted trailing arguments
// are replaced with the constant-expression default node from the declaration,
// so codegen emits exactly len(Args) arguments and never has to know which were
// defaulted. For a builtin, Builtin names it (e.g. "print", "int"); for a user
// call, Func points at the resolved declaration and Mangled is its shell name.
type CallInfo struct {
	Kind      CallKind
	Builtin   string          // set when Kind == CallBuiltin
	Func      *ast.FuncDecl   // set when Kind == CallUser
	Mangled   string          // user function's base mangled shell name (CallUser only)
	Args      []ast.Expr      // full argument list, defaults filled in
	Result    Type            // call result type
	TypeSubst map[string]Type // non-nil for numeric-bounded instantiations; maps param name -> concrete type (or type var for recursive calls)
}

// FuncRef is a resolved function reference (M4): a bare function name used in a
// value context. Mangled is the target function's shell name (mangleFunc(name),
// identical to FuncInfo.Mangled), and Type is the funcref type encoding the
// function's FULL declared arity (defaults do not participate, spec 2.2).
type FuncRef struct {
	Mangled string
	Type    Type
}

// StructFieldInfo is one resolved struct field: its source name and type, in
// declaration order.
type StructFieldInfo struct {
	Name string
	Type Type
}

// EnumInfo is a resolved enum declaration (R2): the declaration node, the source
// name, the defining module's id, and the variant names with their resolved int
// values in declaration order (Variants[i] has value Values[i]). An enum is a
// distinct comparable int-backed type stored in its OWN registry (Info.Enums),
// separate from Structs, so isStructType/isHandle are false for it and == works.
type EnumInfo struct {
	Decl     *ast.EnumDecl
	Name     string
	ID       int
	Variants []string
	Values   []int64
}

// value returns the resolved int value of variant name and whether it exists.
func (e *EnumInfo) value(name string) (int64, bool) {
	for i, v := range e.Variants {
		if v == name {
			return e.Values[i], true
		}
	}
	return 0, false
}

// StructInfo is a resolved struct declaration codegen and the checker share:
// the declaration node, the fields in declaration order, and a name->type
// lookup table.
type StructInfo struct {
	Decl       *ast.StructDecl
	Name       string            // source name (for diagnostics)
	ID         int               // defining module's modid (cross-module struct identity, M8)
	TypeParams []string          // nil for concrete; set for the base generic StructInfo
	Fields     []StructFieldInfo // declaration order
	byName     map[string]Type
}

// FuncInfo is per-function resolved data codegen needs to emit a function.
// Decls lists every variable declared in the function (parameters first, in
// source order, then block-scoped lets in declaration order) so codegen can
// emit `local` declarations and knows the mangled names. Const Vars are NOT
// in Decls; they are inlined at use sites and require no runtime variable.
// Mangled is the function's own shell name (e.g. "__wisp_f_main").
type FuncInfo struct {
	Decl    *ast.FuncDecl
	Mangled string
	Decls   []*Var
}

// Info is the checker result consumed by codegen.
//
// A program is rejected iff len(Errors) > 0. Warnings are informational and
// never gate (spec rules 6, 10).
type Info struct {
	// Types maps each expression node to its resolved type.
	Types map[ast.Expr]Type
	// FoldedValues maps each const-expression node to its compile-time folded
	// value (int64 for int, bool for bool, string for string, or FoldedFloat for
	// a float). Populated by checkConstExpr; codegen reads it to inline the
	// folded literal.
	FoldedValues map[ast.Expr]interface{}
	// Vars maps each variable declaration node (*ast.LetStmt) and each
	// parameter (keyed by the *Var pointer is not possible from the AST, so
	// parameters are reachable via Funcs[...].Decls) to its resolved Var.
	Vars map[*ast.LetStmt]*Var
	// Uses maps each identifier *use* (*ast.Ident in expression position) to
	// the Var it resolves to, so codegen emits the mangled name.
	Uses map[*ast.Ident]*Var
	// FuncRefs maps each *ast.Ident in VALUE position that names a declared
	// function (a function reference, M4) to its resolved form: the function's
	// mangled shell name (the SAME name codegen emits for the definition -- one
	// source of truth) and the funcref Type. genIdent emits the mangled name for
	// such an Ident instead of consulting Info.Uses (which has no entry, because a
	// function is not a Var).
	FuncRefs map[*ast.Ident]*FuncRef
	// MemberFuncRefs maps each namespaced-member *ast.FieldAccess in VALUE
	// position (`ns.member`, Part 3) that names a funcref-able function to its
	// resolved FuncRef. Two cases: a core-module builtin member (funcref-able
	// iff its coreFunc builtin is in the generatable allowlist and it does not
	// take type arguments), resolved to the same __wisp_builtin_<name> wrapper
	// the bare-ident path mints; or an exported, non-generic user function in
	// another module, resolved to that function's own mangled shell name. In
	// both cases the Type is the funcref's signature. genExpr's FieldAccess
	// path emits the mangled name for such a node.
	MemberFuncRefs map[*ast.FieldAccess]*FuncRef
	// ForInVars maps each for-in statement to the Var of its loop binding, so
	// codegen emits the mangled name for the block-scoped element variable.
	ForInVars map[*ast.ForInStmt]*Var
	// CatchVars maps each try statement to the Var of its catch binding `e`
	// (typed error, block-scoped to the handler), so codegen emits the mangled
	// name for the error handle bound in the catch block (M5).
	CatchVars map[*ast.TryStmt]*Var
	// MatchArmVars maps each match arm to the Var of its payload binding (if any).
	// Arms without a payload-binding (None, wildcard, or `_` discard) have no entry.
	MatchArmVars map[*ast.MatchArm]*Var
	// Calls maps each call expression to its resolved form.
	Calls map[*ast.CallExpr]*CallInfo
	// Funcs maps each function declaration to its resolved per-function data.
	Funcs map[*ast.FuncDecl]*FuncInfo
	// Tests maps each `test (...)` declaration to the per-test-body FuncInfo
	// produced while checking its block as a `-> void` scope (its locals/spill
	// temps for codegen's `local` declaration). A test is not a callable function,
	// so it is NOT in Funcs; the test-mode runner codegen reads this map to emit
	// each test body. Empty for a non-`*_test.wisp` file.
	Tests map[*ast.TestDecl]*FuncInfo
	// Structs maps each struct name to its resolved declaration (M3 PR-B).
	Structs map[string]*StructInfo
	// Enums maps each enum's internal token (Name@modid) to its resolved
	// declaration. A SEPARATE registry from Structs (R2): an enum is a comparable
	// int-backed value, not an opaque handle, so isStructType/isHandle are false
	// for it. Codegen needs nothing from this map; variant access lowers via the
	// folded-value inlining of FoldedValues.
	Enums map[string]*EnumInfo
	// Main is the resolved main function (nil if main is missing/invalid).
	Main *ast.FuncDecl
	// MainArgs is true when Main has the `fn main(args: string[])` signature, so
	// codegen materializes "$@" into an array handle at entry (spec 4.5).
	MainArgs bool

	// ConstTable maps each top-level const source name to its folded value and
	// type, populated by the collect-fold pass (Task 3) before body checking.
	// Keyed by source name; single-file scope only (no cross-module consts in
	// this PR). Task 4 reads this map to resolve const references in bodies.
	ConstTable map[string]*ConstEntry

	// ConstVars maps each local (function-body) *ast.ConstStmt to its resolved
	// Var (IsConst == true). Parallel to ForInVars/CatchVars; Task 8
	// LSP iterates this for go-to-def on local consts.
	ConstVars map[*ast.ConstStmt]*Var
	// TopConstVars maps each top-level *ast.ConstDecl to its resolved Var
	// (IsConst == true). Task 8 LSP iterates this for go-to-def on top-level
	// consts. Top-level consts are folded by Pass 3.5; the Var is created there.
	TopConstVars map[*ast.ConstDecl]*Var

	// FinalVars maps each function-body *ast.FinalStmt to its resolved Var
	// (Immutable == true). Parallel to ForInVars/CatchVars. codegen's genFinal
	// looks up the Var by its FinalStmt to emit the runtime `local`. final Vars
	// ARE in FuncInfo.Decls (unlike const).
	FinalVars map[*ast.FinalStmt]*Var

	Errors   []Diagnostic
	Warnings []Diagnostic
}

func newInfo() *Info {
	return &Info{
		Types:          map[ast.Expr]Type{},
		FoldedValues:   map[ast.Expr]interface{}{},
		Vars:           map[*ast.LetStmt]*Var{},
		Uses:           map[*ast.Ident]*Var{},
		FuncRefs:       map[*ast.Ident]*FuncRef{},
		MemberFuncRefs: map[*ast.FieldAccess]*FuncRef{},
		ForInVars:      map[*ast.ForInStmt]*Var{},
		CatchVars:      map[*ast.TryStmt]*Var{},
		MatchArmVars:   map[*ast.MatchArm]*Var{},
		Calls:          map[*ast.CallExpr]*CallInfo{},
		Funcs:          map[*ast.FuncDecl]*FuncInfo{},
		Tests:          map[*ast.TestDecl]*FuncInfo{},
		Structs:        map[string]*StructInfo{},
		Enums:          map[string]*EnumInfo{},
		ConstTable:     map[string]*ConstEntry{},
		ConstVars:      map[*ast.ConstStmt]*Var{},
		TopConstVars:   map[*ast.ConstDecl]*Var{},
		FinalVars:      map[*ast.FinalStmt]*Var{},
	}
}
