// Package ast defines the wisp abstract syntax tree.
//
// Every node exposes Pos() reporting the source position of its first token,
// so the parser and later passes can produce diagnostics and (in M2) source
// maps. Positions are threaded through the AST and must not be discarded by
// later passes.
package ast

import "github.com/mitchellnemitz/wisp/internal/token"

// TypeName is a type annotation. The primitive names are "int", "float",
// "bool", "string", and "void". Composite annotations are encoded into the same
// string (M3): an array type is "[" + elem + "]" (e.g. "[int]", "[[int]]",
// "[Point]"), and a named struct type is the struct's name (e.g. "Point"). The
// encoding is structural, so two equal TypeName strings denote the same type.
type TypeName string

const (
	TypeInt    TypeName = "int"
	TypeFloat  TypeName = "float"
	TypeBool   TypeName = "bool"
	TypeString TypeName = "string"
	TypeVoid   TypeName = "void"
)

// ArrayType encodes an array annotation [elem].
func ArrayType(elem TypeName) TypeName { return "[" + elem + "]" }

// DictType encodes a dict annotation {key: val} as "{" + key + ":" + val + "}".
func DictType(key, val TypeName) TypeName { return "{" + key + ":" + val + "}" }

// OptionalType encodes an optional annotation Optional[elem] as
// "Optional[" + elem + "]". Structural, like array/dict, so two equal TypeName
// strings denote the same optional type.
func OptionalType(elem TypeName) TypeName { return "Optional[" + elem + "]" }

// ResultType encodes a result annotation Result[elem] as "Result[" + elem + "]".
// Single-bracket, like array/optional; the error payload type is fixed to the
// built-in error handle and is not part of the encoding.
func ResultType(elem TypeName) TypeName { return "Result[" + elem + "]" }

// TupleType encodes a tuple annotation "(T1,T2,...,Tn)".
// Mirrors ArrayType, OptionalType, ResultType. n >= 2 is enforced by the parser.
func TupleType(elems []TypeName) TypeName {
	s := "("
	for i, e := range elems {
		if i > 0 {
			s += ","
		}
		s += string(e)
	}
	return TypeName(s + ")")
}

// FuncType encodes a function-reference annotation `fn(P1,P2,...) -> R` as
// "fn(" + join(params, ",") + ")->" + ret (M4). The encoding is structural, so
// two equal TypeName strings denote the same function type and compile-time type
// equality is plain ==.
func FuncType(params []TypeName, ret TypeName) TypeName {
	s := "fn("
	for i, p := range params {
		if i > 0 {
			s += ","
		}
		s += string(p)
	}
	return TypeName(s + ")->" + string(ret))
}

// Node is any AST node.
type Node interface {
	Pos() token.Position
}

// Stmt is a statement node.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression node.
type Expr interface {
	Node
	exprNode()
}

// Program is the root: the top-level struct and function declarations, plus the
// module directives (M8). Imports and Includes are kept in source order.
// Consts holds top-level const declarations in source order.
type Program struct {
	// File is the source filename as threaded into token positions (the path the
	// caller passed to the parser). It lets diagnostics that have no node to
	// anchor to -- e.g. the no-main error on an empty file -- still carry a
	// filename instead of rendering `0:0`.
	File     string
	Structs  []*StructDecl
	Enums    []*EnumDecl
	Aliases  []*TypeAliasDecl
	Funcs    []*FuncDecl
	Consts   []*ConstDecl
	Imports  []*ImportDecl
	Includes []*IncludeDecl
	// Tests holds top-level `test ("name") { ... }` declarations in source order.
	// The parser only produces these for a file whose name ends in `_test.wisp`.
	Tests []*TestDecl
}

func (p *Program) Pos() token.Position {
	if len(p.Funcs) > 0 {
		return p.Funcs[0].Pos()
	}
	return token.Position{}
}

// ImportDecl is `import "owner/repo" [as alias]` (M8). Path is the raw quoted
// owner/repo string (validated by the loader, never a URL); Alias is "" when no
// `as` clause was written (the default namespace is the package's wisp.json name).
type ImportDecl struct {
	KwPos    token.Position
	PathPos  token.Position
	Path     string
	AliasPos token.Position
	Alias    string
}

func (d *ImportDecl) Pos() token.Position { return d.KwPos }

// IncludeDecl is `include "./rel/path.wisp" [as alias]` (M8). Path is the raw
// quoted relative path; Alias is "" when no `as` clause (default namespace is the
// file stem).
type IncludeDecl struct {
	KwPos    token.Position
	PathPos  token.Position
	Path     string
	AliasPos token.Position
	Alias    string
}

func (d *IncludeDecl) Pos() token.Position { return d.KwPos }

// StructField is one field of a struct declaration: a name and its type.
type StructField struct {
	NamePos token.Position
	Name    string
	Type    TypeName
}

// StructDecl is `struct Name { f: T, ... }`. Exported records an `export` modifier
// (M8); ExportPos is the position of the `export` keyword when present.
type StructDecl struct {
	KwPos      token.Position // position of the 'struct' keyword
	NamePos    token.Position
	Name       string
	TypeParams []string // declared type-parameter names, in order; nil for non-generic
	Fields     []StructField
	Exported   bool
	ExportPos  token.Position
	Multiline  bool // true iff a real newline separated fields in the body (not `;`)
}

func (d *StructDecl) Pos() token.Position { return d.KwPos }

// EnumVariant is one variant of an enum declaration: a name and an optional
// explicit value. Value is nil for an implicit variant (C-style auto-increment,
// resolved by the checker). When present, Value is a general expression node
// (the checker restricts it to an integer literal with an optional leading `-`).
// NamePos is the variant's source position (the name token).
type EnumVariant struct {
	Name       string
	NamePos    token.Position
	Value      Expr     // nil if implicit
	Payload    TypeName // "" if the variant carries no payload (tagged-union enum)
	PayloadPos token.Position
}

// EnumDecl is `enum Name { V1[ = expr], V2, ... }`. Variants are comma- and/or
// newline-separated with an optional trailing comma; at least one is required.
// Multiline is true iff a real newline (not `;`) separated variants in the body,
// matching the StructDecl rule, so wisp fmt can preserve the layout. Exported
// records an `export` modifier, mirroring StructDecl; ExportPos is the position
// of the `export` keyword when present.
type EnumDecl struct {
	KwPos      token.Position // position of the 'enum' keyword
	NamePos    token.Position
	Name       string
	Backing    TypeName // "" = bare (tagged-union); else the value-enum backing type name
	BackingPos token.Position
	TypeParams []string // non-empty only for a (rejected) generic user enum
	Variants   []EnumVariant
	Multiline  bool
	Exported   bool
	ExportPos  token.Position
}

func (d *EnumDecl) Pos() token.Position { return d.KwPos }

// TypeAliasDecl is a top-level `type Name = T` transparent type-alias
// declaration. Name is a pure structural synonym for the type annotation Type;
// the checker resolves it to Type's underlying Type, so the alias name never
// reaches type-equality, codegen, or the runtime. KwPos is the `type` keyword
// position; NamePos is the alias name; TypePos is the first RHS-annotation token
// (used to locate RHS resolution errors precisely).
type TypeAliasDecl struct {
	KwPos   token.Position
	NamePos token.Position
	Name    string
	TypePos token.Position
	Type    TypeName
}

func (d *TypeAliasDecl) Pos() token.Position { return d.KwPos }

// Param is a function parameter. Default is nil when the parameter has no
// default; when present it is a constant-expression node (Section 10.3).
type Param struct {
	NamePos token.Position
	Name    string
	Type    TypeName
	Default Expr
}

func (p *Param) Pos() token.Position { return p.NamePos }

// FuncDecl is a function declaration. Exported records an `export` modifier (M8);
// ExportPos is the position of the `export` keyword when present.
type FuncDecl struct {
	KwPos           token.Position // position of the 'fn' keyword
	Name            string
	TypeParams      []string          // declared type-parameter names in order; nil for a non-generic fn
	TypeParamBounds map[string]string // type-param name -> bound ("comparable" or "numeric"); nil/absent = unbounded
	Params          []Param
	RetType         TypeName
	Body            []Stmt
	Exported        bool
	ExportPos       token.Position
}

func (f *FuncDecl) Pos() token.Position { return f.KwPos }

// TestDecl is a `test ("name") { ... }` declaration. It is a top-level construct,
// only valid in a `*_test.wisp` file. Name is the static string-literal test name
// (used for reporting and filtering); NamePos is the position of that literal.
// Body is the test block, type-checked like a `-> void` function body. KwPos is
// the position of the `test` keyword.
type TestDecl struct {
	KwPos   token.Position // position of the 'test' keyword
	NamePos token.Position
	Name    string
	Body    []Stmt
}

func (d *TestDecl) Pos() token.Position { return d.KwPos }

// --- Statements ---

// ConstDecl is a top-level `const NAME: Type = <const-expr>` declaration.
// It is collected in Program.Consts parallel to Structs/Funcs.
// KwPos is the position of the `const` keyword; NamePos is the name position.
// Exported records an `export` modifier (export const, M8 cross-module consts);
// ExportPos is the position of the `export` keyword when present.
type ConstDecl struct {
	KwPos     token.Position
	NamePos   token.Position
	Name      string
	Type      TypeName
	Value     Expr
	Exported  bool
	ExportPos token.Position
}

func (d *ConstDecl) Pos() token.Position { return d.KwPos }

// ConstStmt is a function-body `const NAME: Type = <const-expr>` statement.
// KwPos is the position of the `const` keyword; NamePos is the name position.
type ConstStmt struct {
	KwPos   token.Position
	NamePos token.Position
	Name    string
	Type    TypeName
	Value   Expr
}

func (s *ConstStmt) Pos() token.Position { return s.KwPos }
func (s *ConstStmt) stmtNode()           {}

// FinalStmt is a function-body `final NAME: Type = <expr>` statement.
// final is function-local only; a top-level final is a parse error.
// KwPos is the position of the `final` keyword; NamePos is the name position.
type FinalStmt struct {
	KwPos   token.Position
	NamePos token.Position
	Name    string
	Type    TypeName
	Value   Expr
}

func (s *FinalStmt) Pos() token.Position { return s.KwPos }
func (s *FinalStmt) stmtNode()           {}

// LetStmt is `let name: type = value`.
type LetStmt struct {
	KwPos token.Position
	Name  string
	Type  TypeName
	Value Expr
}

func (s *LetStmt) Pos() token.Position { return s.KwPos }
func (s *LetStmt) stmtNode()           {}

// TupleBindSlot is one slot of a tuple-destructuring pattern. A binding slot
// carries a Name and a mandatory Type (Blank false). A discard slot is bare
// `_` (Blank true) with an OPTIONAL Type: empty Type means a bare `_` that
// imposes no constraint on its element; a non-empty Type is a checked discard.
// Pos is the slot's source position so the checker can emit slot-located
// diagnostics (a bound name's Var.Pos is set from it).
type TupleBindSlot struct {
	Name  string
	Blank bool
	Type  TypeName // empty for a bare `_`
	Pos   token.Position
}

// TupleBindStmt is a tuple-destructuring `let`/`final` binding:
// `let (a: int, b: string) = <expr>` (Final false) or the `final` form
// (Final true). Slots has length k >= 2. Value is the tuple-typed RHS,
// evaluated once. KwPos is the position of the `let`/`final` keyword.
type TupleBindStmt struct {
	KwPos token.Position
	Final bool
	Slots []TupleBindSlot
	Value Expr
}

func (s *TupleBindStmt) Pos() token.Position { return s.KwPos }
func (s *TupleBindStmt) stmtNode()           {}

// AssignStmt is `name = value`.
type AssignStmt struct {
	NamePos token.Position
	Name    string
	Value   Expr
}

func (s *AssignStmt) Pos() token.Position { return s.NamePos }
func (s *AssignStmt) stmtNode()           {}

// FieldAssignStmt is `target.Field = value` (M3 struct field assignment).
// Target is the struct-handle expression.
type FieldAssignStmt struct {
	Target Expr
	DotPos token.Position
	Field  string
	Value  Expr
}

func (s *FieldAssignStmt) Pos() token.Position { return s.Target.Pos() }
func (s *FieldAssignStmt) stmtNode()           {}

// IndexAssignStmt is `target[Index] = value` (M3 array element assignment).
// Target is the array-handle expression.
type IndexAssignStmt struct {
	Target  Expr
	LBrkPos token.Position
	Index   Expr
	Value   Expr
}

func (s *IndexAssignStmt) Pos() token.Position { return s.Target.Pos() }
func (s *IndexAssignStmt) stmtNode()           {}

// ForInStmt is `for (x in Coll) { body }`. Var is the loop binding (block
// scoped, like the C-style for-init); Coll is the array (M3 PR-B) or dict
// (PR-C) being iterated.
type ForInStmt struct {
	KwPos  token.Position
	VarPos token.Position
	Var    string
	Coll   Expr
	Body   []Stmt
}

func (s *ForInStmt) Pos() token.Position { return s.KwPos }
func (s *ForInStmt) stmtNode()           {}

// ReturnStmt is `return [value]`. Value is nil for a void return.
type ReturnStmt struct {
	KwPos token.Position
	Value Expr
}

func (s *ReturnStmt) Pos() token.Position { return s.KwPos }
func (s *ReturnStmt) stmtNode()           {}

// ElseIf is one `else if (cond) { body }` arm.
type ElseIf struct {
	KwPos token.Position // the `else` keyword of this else-if arm
	Cond  Expr
	Body  []Stmt
}

// IfStmt is an if / else-if* / else? chain. Else is nil when absent.
type IfStmt struct {
	KwPos   token.Position
	Cond    Expr
	Then    []Stmt
	ElseIfs []ElseIf
	ElsePos token.Position // the final `else` keyword; zero when Else is nil
	Else    []Stmt
}

func (s *IfStmt) Pos() token.Position { return s.KwPos }
func (s *IfStmt) stmtNode()           {}

// MatchPattern is the sealed interface for match arm patterns.
type MatchPattern interface{ patternNode() }

// ConstructorPat is a `Variant(name)` or bare `Variant` pattern.
// Name is empty when the variant carries no payload (e.g. None).
// Name is "_" when the payload is present but explicitly discarded.
type ConstructorPat struct {
	Variant    string
	VariantPos token.Position
	Name       string
	NamePos    token.Position
}

func (p *ConstructorPat) patternNode() {}

// WildcardPat is the `_` arm-level discard. It must be the last arm and
// covers all remaining unmatched variants.
type WildcardPat struct {
	Pos token.Position
}

func (p *WildcardPat) patternNode() {}

// MatchArm is one arm of a match: case <Pattern> { Body }.
type MatchArm struct {
	Pattern MatchPattern
	CasePos token.Position
	Body    []Stmt
}

// MatchStmt is `match (scrutinee) { arm... }`. Exhaustiveness is required.
type MatchStmt struct {
	KwPos     token.Position
	Scrutinee Expr
	Arms      []*MatchArm
}

func (s *MatchStmt) Pos() token.Position { return s.KwPos }
func (s *MatchStmt) stmtNode()           {}

// WhileStmt is `while (cond) { body }`.
type WhileStmt struct {
	KwPos token.Position
	Cond  Expr
	Body  []Stmt
}

func (s *WhileStmt) Pos() token.Position { return s.KwPos }
func (s *WhileStmt) stmtNode()           {}

// ForStmt is `for (init; cond; post) { body }`. Init and Post may be nil.
type ForStmt struct {
	KwPos token.Position
	Init  Stmt
	Cond  Expr
	Post  Stmt
	Body  []Stmt
}

func (s *ForStmt) Pos() token.Position { return s.KwPos }
func (s *ForStmt) stmtNode()           {}

// SwitchCase is one `case v, w { body }` clause.
type SwitchCase struct {
	Values []Expr
	Body   []Stmt
}

// SwitchStmt is `switch (subject) { case... default... }`. Default is nil when
// absent in source (the parser permits absence; the type checker enforces it).
type SwitchStmt struct {
	KwPos   token.Position
	Subject Expr
	Cases   []SwitchCase
	Default []Stmt
}

func (s *SwitchStmt) Pos() token.Position { return s.KwPos }
func (s *SwitchStmt) stmtNode()           {}

// BreakStmt is `break`.
type BreakStmt struct {
	KwPos token.Position
}

func (s *BreakStmt) Pos() token.Position { return s.KwPos }
func (s *BreakStmt) stmtNode()           {}

// ContinueStmt is `continue`.
type ContinueStmt struct {
	KwPos token.Position
}

func (s *ContinueStmt) Pos() token.Position { return s.KwPos }
func (s *ContinueStmt) stmtNode()           {}

// ExprStmt is an expression used as a statement. In M1 the only expression
// valid at statement level is a call.
type ExprStmt struct {
	X Expr
}

func (s *ExprStmt) Pos() token.Position { return s.X.Pos() }
func (s *ExprStmt) stmtNode()           {}

// ThrowStmt is `throw <expr>` (M5). X must be of type error (checker). It is a
// terminating statement for all-paths-return, like return.
type ThrowStmt struct {
	KwPos token.Position
	X     Expr
}

func (s *ThrowStmt) Pos() token.Position { return s.KwPos }
func (s *ThrowStmt) stmtNode()           {}

// TryStmt is `try { Body } catch (CatchVar) { Catch } [finally { Finally }]`
// (M5). CatchVar is the error binding in the handler scope. HasFinally records
// whether a finally clause was written (an empty finally block is still
// present); Finally is the (possibly empty) cleanup body.
type TryStmt struct {
	KwPos       token.Position
	Body        []Stmt
	CatchPos    token.Position
	CatchVarPos token.Position
	CatchVar    string
	Catch       []Stmt
	HasFinally  bool
	FinallyPos  token.Position // the `finally` keyword; zero when HasFinally is false
	Finally     []Stmt
}

func (s *TryStmt) Pos() token.Position { return s.KwPos }
func (s *TryStmt) stmtNode()           {}

// --- Expressions ---

// IntLit is an integer literal; Raw holds the source digits (no sign).
type IntLit struct {
	LitPos token.Position
	Raw    string
}

func (e *IntLit) Pos() token.Position { return e.LitPos }
func (e *IntLit) exprNode()           {}

// FloatLit is a float literal; Raw holds the source text <digits>.<digits>
// (no sign; a leading unary minus is a UnaryExpr over this node).
type FloatLit struct {
	LitPos token.Position
	Raw    string
}

func (e *FloatLit) Pos() token.Position { return e.LitPos }
func (e *FloatLit) exprNode()           {}

// BoolLit is `true` or `false`.
type BoolLit struct {
	LitPos token.Position
	Value  bool
}

func (e *BoolLit) Pos() token.Position { return e.LitPos }
func (e *BoolLit) exprNode()           {}

// StringPart is one piece of a string literal: either literal text (Expr nil)
// or an embedded interpolation expression (Expr non-nil).
type StringPart struct {
	Text string // valid when Expr == nil
	Expr Expr   // valid when non-nil
}

// IsText reports whether the part is a literal text piece.
func (p StringPart) IsText() bool { return p.Expr == nil }

// StringLit models both forms. A single-quoted literal is one text part. A
// double-quoted literal is a sequence of text and interpolation parts.
type StringLit struct {
	LitPos token.Position
	Parts  []StringPart
}

func (e *StringLit) Pos() token.Position { return e.LitPos }
func (e *StringLit) exprNode()           {}

// Ident is a variable reference.
type Ident struct {
	NamePos token.Position
	Name    string
}

func (e *Ident) Pos() token.Position { return e.NamePos }
func (e *Ident) exprNode()           {}

// UnaryExpr is `Op X` (Op is token.Minus or token.Bang).
type UnaryExpr struct {
	OpPos token.Position
	Op    token.Kind
	X     Expr
}

func (e *UnaryExpr) Pos() token.Position { return e.OpPos }
func (e *UnaryExpr) exprNode()           {}

// BinaryExpr is `L Op R`.
type BinaryExpr struct {
	OpPos token.Position
	Op    token.Kind
	L     Expr
	R     Expr
}

func (e *BinaryExpr) Pos() token.Position { return e.L.Pos() }
func (e *BinaryExpr) exprNode()           {}

// CallExpr is `Callee(Args...)`. Callee is the callee EXPRESSION (M4): a bare
// identifier `f(...)`, a conversion/builtin name, a field `s.op(...)`, an array
// element `fns[0](...)`, or any other expression that evaluates to a function
// reference `getOp()(...)`. The checker resolves whether the callee names a
// declared function (direct call), a local funcref (indirect call), or a
// builtin. CalleeName is set when Callee is a bare *Ident, giving the
// fast-path name for direct/builtin resolution; it is "" otherwise.
type CallExpr struct {
	CalleePos  token.Position
	Callee     Expr
	CalleeName string
	// TypeArgs are explicit call-site type arguments `name[T1, T2](args)` (M9); nil
	// when absent (the common case). They are consumed only by a direct or qualified
	// user-function call; every other callee form rejects them. TypeArg is not an
	// Expr, so no expression walker recurses into it.
	TypeArgs []TypeArg
	Args     []Expr
}

func (e *CallExpr) Pos() token.Position { return e.CalleePos }
func (e *CallExpr) exprNode()           {}

// TypeArg is one explicit call-site type argument: its type-name encoding plus the
// source position of its first token, for located diagnostics.
type TypeArg struct {
	Name TypeName
	Pos  token.Position
}

// StructLitField is one `name: value` in a struct construction.
type StructLitField struct {
	NamePos token.Position
	Name    string
	Value   Expr
}

// StructLit is `Name { f: v, ... }` (M3 struct construction). It evaluates to a
// fresh struct handle. Namespace is "" for a local struct; for a qualified literal
// `ns.Type { ... }` (M8) it is the namespace alias.
type StructLit struct {
	NamePos   token.Position
	Name      string
	Fields    []StructLitField
	Namespace string
	Multiline bool // true iff a real newline separated fields in the body (not `;`)
}

func (e *StructLit) Pos() token.Position { return e.NamePos }
func (e *StructLit) exprNode()           {}

// ArrayLit is `[a, b, c]` (M3 array construction). It evaluates to a fresh array
// handle. An empty literal `[]` has no element to infer from, so its type comes
// from the surrounding annotation (handled by the checker).
type ArrayLit struct {
	LBrkPos   token.Position
	Elems     []Expr
	Multiline bool // true iff a real newline separated elements (not `;`)
}

func (e *ArrayLit) Pos() token.Position { return e.LBrkPos }
func (e *ArrayLit) exprNode()           {}

// TupleLit is `(e1, e2, ..., en)`, n >= 2. Elems are in source order.
// It mirrors ArrayLit and is produced by parsePrimary when a comma follows
// the first element inside parentheses.
type TupleLit struct {
	LParenPos token.Position
	Elems     []Expr
}

func (e *TupleLit) Pos() token.Position { return e.LParenPos }
func (e *TupleLit) exprNode()           {}

// DictLitEntry is one `key: value` in a dict construction.
type DictLitEntry struct {
	Key   Expr
	Colon token.Position
	Value Expr
}

// DictLit is `{ k: v, ... }` (M3 PR-C dict construction). It evaluates to a
// fresh dict handle. An empty literal `{}` has no entry to infer from, so its
// type comes from the surrounding annotation (handled by the checker).
type DictLit struct {
	LBrace    token.Position
	Entries   []DictLitEntry
	Multiline bool // true iff a real newline separated entries in the body (not `;`)
}

func (e *DictLit) Pos() token.Position { return e.LBrace }
func (e *DictLit) exprNode()           {}

// FieldAccess is `X.Field` (M3 struct field read).
type FieldAccess struct {
	X      Expr
	DotPos token.Position
	Field  string
}

func (e *FieldAccess) Pos() token.Position { return e.X.Pos() }
func (e *FieldAccess) exprNode()           {}

// IndexExpr is `X[Index]` (M3 array element read).
type IndexExpr struct {
	X       Expr
	LBrkPos token.Position
	Index   Expr
}

func (e *IndexExpr) Pos() token.Position { return e.X.Pos() }
func (e *IndexExpr) exprNode()           {}
