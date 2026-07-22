package types

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// collectEnumNames is pass 1 of enum resolution: it registers each enum's name,
// mode (value vs tagged), backing type, and -- for a value enum -- its resolved
// per-variant backing constants and uniqueness. Payload TYPES are resolved in
// pass 2 (checkEnumPayloads) so a payload may reference a forward or mutual enum
// (FR-014). collectStructs+collectEnumNames must both run before
// checkStructFields+checkEnumPayloads. The caller sets c.cur = ctx first.
func (c *checker) collectEnumNames(ctx *moduleCtx) {
	for _, ed := range ctx.prog.Enums {
		if isReservedIdent(ed.Name) {
			c.errf(ed.NamePos, "%q uses the reserved \"__\" namespace and cannot be an enum name", ed.Name)
		}
		if isReservedName(ed.Name) {
			c.errf(ed.NamePos, "%q is a reserved builtin or constant name and cannot be an enum name", ed.Name)
		}
		if isPrimitiveTypeName(ed.Name) {
			c.errf(ed.NamePos, "%q is a built-in type name and cannot be an enum name", ed.Name)
		}
		if _, dup := ctx.enums[ed.Name]; dup {
			c.errf(ed.NamePos, "enum %q is declared more than once", ed.Name)
			continue
		}
		if _, clash := ctx.structs[ed.Name]; clash {
			c.errf(ed.NamePos, "%q is already declared as a struct; an enum and a struct cannot share a name", ed.Name)
			continue
		}
		// FR-019: a generic user enum is rejected here with a located type error
		// (the parser accepted the `[T]` type-param list so this message, not a
		// bare parse error, fires).
		if len(ed.TypeParams) > 0 {
			c.errf(ed.NamePos, "generic user enum %q is not supported; a user tagged-union enum must have concrete payload types", ed.Name)
			// fall through and register it (as tagged) for error recovery
		}

		ei := &EnumInfo{Decl: ed, Name: ed.Name, ID: ctx.id}
		hasPayloadVariant := false
		for _, v := range ed.Variants {
			if v.Payload != "" {
				hasPayloadVariant = true
			}
		}
		if ed.Backing != "" {
			c.collectValueEnum(ctx, ed, ei)
		} else {
			c.collectTaggedEnum(ctx, ed, ei, hasPayloadVariant)
		}

		c.info.Enums[string(internalEnumName(ed.Name, ctx.id))] = ei
		ctx.enums[ed.Name] = ei
		if ed.Exported {
			ctx.exportedEnums[ed.Name] = true
		}
	}
}

// collectValueEnum resolves a `: int|string|bool` value enum's backing type and
// per-variant constants (FR-002a defaults + uniqueness), and rejects a payload on
// any variant (FR-022) and a non-int/string/bool backing such as float (SC-039).
func (c *checker) collectValueEnum(ctx *moduleCtx, ed *ast.EnumDecl, ei *EnumInfo) {
	ei.Kind = EnumValue
	switch ed.Backing {
	case ast.TypeInt:
		ei.Backing = Int
	case ast.TypeString:
		ei.Backing = String
	case ast.TypeBool:
		ei.Backing = Bool
	default:
		// SC-039: float and any other backing rejected; the enum stays a
		// (malformed) value enum with an Invalid backing for error recovery.
		ei.Backing = Invalid
		c.errf(ed.BackingPos, "enum backing type must be int, string, or bool, got %s", ed.Backing)
	}
	seenName := map[string]bool{}
	nextInt := int64(0)
	for _, v := range ed.Variants {
		if isReservedIdent(v.Name) {
			c.errf(v.NamePos, "%q uses the reserved \"__\" namespace and cannot be a variant name", v.Name)
		}
		if seenName[v.Name] {
			c.errf(v.NamePos, "variant %q is declared more than once in enum %q", v.Name, ed.Name)
			continue
		}
		seenName[v.Name] = true
		// FR-022: a value enum's variant MUST NOT carry a payload.
		if v.Payload != "" {
			c.errf(v.PayloadPos, "variant %q of value enum %q must not carry a payload; a payload variant requires a bare (tagged-union) enum with no backing type", v.Name, ed.Name)
		}
		var cv interface{}
		switch ei.Backing {
		case Int:
			if v.Value != nil {
				iv, ok := c.enumLiteralValue(v.Value)
				if !ok {
					nextInt++
					ei.Variants = append(ei.Variants, v.Name)
					ei.Consts = append(ei.Consts, nil)
					ei.Payloads = append(ei.Payloads, Invalid)
					continue
				}
				nextInt = iv
			}
			cv = nextInt
			nextInt++
		case String:
			if v.Value != nil {
				sv, ok := c.enumStringLiteralValue(v.Value)
				if !ok {
					ei.Variants = append(ei.Variants, v.Name)
					ei.Consts = append(ei.Consts, nil)
					ei.Payloads = append(ei.Payloads, Invalid)
					continue
				}
				cv = sv
			} else {
				cv = v.Name // FR-002a: string default is the variant's own name
			}
		case Bool:
			// FR-002a: bool has NO default; every variant needs an explicit value.
			if v.Value == nil {
				c.errf(v.NamePos, "variant %q of bool enum %q must declare an explicit = true or = false", v.Name, ed.Name)
				ei.Variants = append(ei.Variants, v.Name)
				ei.Consts = append(ei.Consts, nil)
				ei.Payloads = append(ei.Payloads, Invalid)
				continue
			}
			bv, ok := c.enumBoolLiteralValue(v.Value)
			if !ok {
				ei.Variants = append(ei.Variants, v.Name)
				ei.Consts = append(ei.Consts, nil)
				ei.Payloads = append(ei.Payloads, Invalid)
				continue
			}
			cv = bv
		default:
			cv = nil // Invalid backing already reported
		}
		ei.Variants = append(ei.Variants, v.Name)
		ei.Consts = append(ei.Consts, cv)
		ei.Payloads = append(ei.Payloads, Invalid)
	}
	// FR-002a: uniqueness across all resolved constants, located at the second use.
	seenVal := map[interface{}]string{}
	for i, name := range ei.Variants {
		cv := ei.Consts[i]
		if cv == nil {
			continue
		}
		if first, ok := seenVal[cv]; ok {
			c.errf(variantPos(ed, name), "variant %q has duplicate value (already used by %q); enum values must be distinct", name, first)
			continue
		}
		seenVal[cv] = name
	}
}

// collectTaggedEnum resolves a bare enum's mode as a tagged union: FR-002
// requires at least one payload variant; FR-022 rejects an explicit value on a
// bare enum (with FR-022 taking precedence over FR-002 for the old-style
// `enum X { A = 0 }` case, so exactly one diagnostic fires).
func (c *checker) collectTaggedEnum(ctx *moduleCtx, ed *ast.EnumDecl, ei *EnumInfo, hasPayloadVariant bool) {
	ei.Kind = EnumTagged
	ei.Backing = Invalid
	sawExplicitValue := false
	seenName := map[string]bool{}
	for _, v := range ed.Variants {
		if isReservedIdent(v.Name) {
			c.errf(v.NamePos, "%q uses the reserved \"__\" namespace and cannot be a variant name", v.Name)
		}
		if seenName[v.Name] {
			c.errf(v.NamePos, "variant %q is declared more than once in enum %q", v.Name, ed.Name)
			continue
		}
		seenName[v.Name] = true
		if v.Value != nil && !sawExplicitValue {
			// FR-022 (precedence over FR-002): an explicit value pins value-enum
			// intent; direct the author to add a `: <type>` backing. Reported once.
			c.errf(v.NamePos, "variant %q of bare enum %q must not carry an explicit value; add a backing annotation `enum %s: int { ... }` to declare a value enum", v.Name, ed.Name, ed.Name)
			sawExplicitValue = true
		}
		// Payload TYPE resolved in pass 2; record the name and placeholders now.
		ei.Variants = append(ei.Variants, v.Name)
		ei.Consts = append(ei.Consts, nil)
		ei.Payloads = append(ei.Payloads, Invalid)
	}
	// FR-002: a bare enum needs at least one payload variant (unless it already
	// erred on an explicit value, in which case FR-022 governs and is the single
	// error, so suppress the FR-002 message).
	if !hasPayloadVariant && !sawExplicitValue {
		c.errf(ed.NamePos, "bare enum %q has no payload variant; add a backing annotation `enum %s: int { ... }` for a value enum, or give a variant a payload like `Variant(int)`", ed.Name, ed.Name)
	}
}

// checkEnumPayloads is pass 2: it resolves each tagged-union variant's payload
// type now that every enum and struct name in every module is registered, so a
// payload may reference a forward or mutual enum (FR-014/SC-041). A value enum
// has no payloads and is skipped. The caller sets c.cur = ctx first.
func (c *checker) checkEnumPayloads(ctx *moduleCtx) {
	for _, ed := range ctx.prog.Enums {
		if ed.Backing != "" {
			continue // value enum: no payloads
		}
		ei := ctx.enums[ed.Name]
		if ei == nil {
			continue // duplicate/malformed; already reported
		}
		for i, v := range ed.Variants {
			if i >= len(ei.Payloads) {
				break
			}
			if v.Payload == "" {
				ei.Payloads[i] = Invalid // no-payload variant
				continue
			}
			pt := c.resolveType(v.Payload, v.PayloadPos)
			ei.Payloads[i] = pt
		}
	}
}

// isValueEnum reports whether t names a declared value enum (: int/string/bool).
func (c *checker) isValueEnum(t Type) bool {
	ei, ok := c.info.Enums[string(t)]
	return ok && ei.Kind == EnumValue
}

// isTaggedEnum reports whether t names a declared tagged-union enum.
func (c *checker) isTaggedEnum(t Type) bool {
	ei, ok := c.info.Enums[string(t)]
	return ok && ei.Kind == EnumTagged
}

// enumLiteralValue evaluates an explicit enum variant value. v1 restricts it to
// an integer literal with an optional leading `-` (R2); any other expression is a
// located "enum value must be an integer literal" error.
func (c *checker) enumLiteralValue(e ast.Expr) (int64, bool) {
	switch n := e.(type) {
	case *ast.IntLit:
		v, err := parseWispInt(n.Raw, false)
		if err != nil {
			c.errf(n.LitPos, "integer literal out of range: %q", n.Raw)
			return 0, false
		}
		return v, true
	case *ast.UnaryExpr:
		if n.Op == token.Minus {
			if il, ok := n.X.(*ast.IntLit); ok {
				v, err := parseWispInt(il.Raw, true)
				if err != nil {
					c.errf(il.LitPos, "integer literal out of range: %q", "-"+il.Raw)
					return 0, false
				}
				return v, true
			}
		}
	}
	c.errf(e.Pos(), "enum value must be an integer literal (optionally negated)")
	return 0, false
}

// enumStringLiteralValue evaluates an explicit string-backed enum variant
// value. ast.StringLit has no scalar Value field; it holds Parts []StringPart
// (interpolation-capable). Reuse the existing stringLitText helper
// (internal/types/stdlib.go:653), which concatenates the text parts and
// returns ok=false if any part is an interpolation expression -- the same
// text-only fold const string handling uses.
func (c *checker) enumStringLiteralValue(e ast.Expr) (string, bool) {
	if s, ok := stringLitText(e); ok {
		return s, true
	}
	c.errf(e.Pos(), "enum value must be a string literal (no interpolation)")
	return "", false
}

// enumBoolLiteralValue evaluates an explicit bool-backed enum variant value.
func (c *checker) enumBoolLiteralValue(e ast.Expr) (bool, bool) {
	if bl, ok := e.(*ast.BoolLit); ok {
		return bl.Value, true
	}
	c.errf(e.Pos(), "enum value must be true or false")
	return false, false
}

// variantPos returns the source position of variant name in ed (for diagnostics).
func variantPos(ed *ast.EnumDecl, name string) token.Position {
	for _, v := range ed.Variants {
		if v.Name == name {
			return v.NamePos
		}
	}
	return ed.NamePos
}

// enumTypeOfBase reports the enum token a FieldAccess base names, when the base
// is a bare identifier that resolves to an enum type and is NOT shadowed by an
// in-scope local variable or a module namespace alias (R3 pinned precedence).
func (c *checker) enumTypeOfBase(n *ast.FieldAccess) (Type, *EnumInfo, bool) {
	id, ok := n.X.(*ast.Ident)
	if !ok {
		return Invalid, nil, false
	}
	if c.lookup(id.Name) != nil {
		return Invalid, nil, false // a local variable shadows the enum interpretation
	}
	if _, isNS := c.cur.namespaces[id.Name]; isNS {
		return Invalid, nil, false // a namespace alias keeps higher precedence
	}
	ei, ok := c.cur.enums[id.Name]
	if !ok {
		return Invalid, nil, false
	}
	return internalEnumName(id.Name, c.cur.id), ei, true
}

// checkEnumVariantAccess handles `Color.Red` when the base names an enum type. It
// types the access to the enum, folds it to the variant's int value, and records
// the fold in info.FoldedValues (R3/R6). Returns (type, true) when handled; an
// unknown variant is a located error (returns Invalid, true). When the base is not
// an enum type it returns (_, false) so the caller falls through to the struct
// field path.
func (c *checker) checkEnumVariantAccess(n *ast.FieldAccess) (Type, bool) {
	tok, ei, ok := c.enumTypeOfBase(n)
	if !ok {
		return Invalid, false
	}
	if ei.Kind == EnumTagged {
		// A bare `Expr.Unit` / `Expr.IntLit` reference is construction-or-error;
		// handled by checkFieldAccess (Task 7). Signal "not a value-enum fold".
		return Invalid, false
	}
	cv, found := ei.constValue(n.Field)
	if !found {
		c.errf(n.DotPos, "enum %q has no variant %q", ei.Name, n.Field)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	c.info.Types[n] = tok
	c.info.FoldedValues[n] = cv
	return tok, true
}

// checkIntEnumCall handles to_int(e) for an enum operand: an int-backed value
// enum is identity; a non-int value enum is directed to its own backing; a
// tagged-union enum is rejected (handle opacity).
func (c *checker) checkIntEnumCall(n *ast.CallExpr) (Type, bool) {
	if len(n.Args) != 1 {
		return Invalid, false
	}
	at := c.checkExpr(n.Args[0])
	if !c.isEnumType(at) {
		return Invalid, false
	}
	ei := c.info.Enums[string(at)]
	if ei.Kind == EnumTagged {
		c.errf(n.Args[0].Pos(), "to_int() is not defined for enum %s (a tagged-union enum has no scalar value)", disp(at))
		return Invalid, true
	}
	if ei.Backing != Int {
		c.errf(n.Args[0].Pos(), "to_int() is not defined for enum %s; use to_%s() for its backing value", disp(at), backingName(ei.Backing))
		return Invalid, true
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "to_int", Args: n.Args, Result: Int}
	return Int, true
}

// checkStringEnumCall handles to_string(e) for an enum operand: a string-backed
// value enum returns its backing string; other value enums are directed to their
// backing; a tagged-union enum is rejected.
func (c *checker) checkStringEnumCall(n *ast.CallExpr) (Type, bool) {
	if len(n.Args) != 1 {
		return Invalid, false
	}
	at := c.checkExpr(n.Args[0])
	if !c.isEnumType(at) {
		return Invalid, false
	}
	ei := c.info.Enums[string(at)]
	if ei.Kind == EnumTagged {
		c.errf(n.Args[0].Pos(), "to_string() is not defined for enum %s (variant-name rendering is not supported)", disp(at))
		return Invalid, true
	}
	if ei.Backing != String {
		c.errf(n.Args[0].Pos(), "to_string() is not defined for enum %s; use to_%s() for its backing value", disp(at), backingName(ei.Backing))
		return Invalid, true
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "to_string", Args: n.Args, Result: String}
	return String, true
}

// checkBoolEnumCall handles to_bool(e) for an enum operand: a bool-backed value
// enum returns its backing bool; others are rejected.
func (c *checker) checkBoolEnumCall(n *ast.CallExpr) (Type, bool) {
	if len(n.Args) != 1 {
		return Invalid, false
	}
	at := c.checkExpr(n.Args[0])
	if !c.isEnumType(at) {
		return Invalid, false
	}
	ei := c.info.Enums[string(at)]
	if ei.Kind == EnumTagged {
		c.errf(n.Args[0].Pos(), "to_bool() is not defined for enum %s (a tagged-union enum has no scalar value)", disp(at))
		return Invalid, true
	}
	if ei.Backing != Bool {
		c.errf(n.Args[0].Pos(), "to_bool() is not defined for enum %s; use to_%s() for its backing value", disp(at), backingName(ei.Backing))
		return Invalid, true
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "to_bool", Args: n.Args, Result: Bool}
	return Bool, true
}

func backingName(t Type) string {
	switch t {
	case Int:
		return "int"
	case String:
		return "string"
	case Bool:
		return "bool"
	}
	return "?"
}

// isEnumType reports whether t names a declared enum. The separate enum registry
// makes this decidable; it is FALSE for structs/handles/primitives.
func (c *checker) isEnumType(t Type) bool {
	_, ok := c.info.Enums[string(t)]
	return ok
}

// variantExists reports whether ei declares a variant named name.
func variantExists(ei *EnumInfo, name string) bool {
	for _, v := range ei.Variants {
		if v == name {
			return true
		}
	}
	return false
}

// qualifiedEnumOfBase resolves the cross-module construction base `ns.Enum` of a
// callee `ns.Enum.Variant`, mirroring checkQualifiedEnumVariantAccess's target-
// module resolution (internal/types/expr.go). It returns the internal enum
// token and its EnumInfo when fa.X is `ns.Enum` naming an enum EXPORTED by the
// target module; otherwise (_, _, false) so the caller falls through to its
// other paths -- which surface the same "not a namespace" / "not exported"
// diagnostics the bare qualified-access path already produces, so no error is
// duplicated here.
func (c *checker) qualifiedEnumOfBase(fa *ast.FieldAccess) (Type, *EnumInfo, bool) {
	inner, isFA := fa.X.(*ast.FieldAccess)
	if !isFA {
		return Invalid, nil, false
	}
	enumName, modid, ok := c.qualifiedNsTarget(inner)
	if !ok {
		return Invalid, nil, false
	}
	tctx := c.modCtx[modid]
	ei, isEnum := tctx.enums[enumName]
	if !isEnum || !tctx.exportedEnums[enumName] {
		return Invalid, nil, false
	}
	return internalEnumName(enumName, modid), ei, true
}

// checkEnumConstruct type-checks a tagged-union construction call
// `Enum.Variant(arg)` (callee is a FieldAccess naming an enum variant). It
// validates arity and the payload type and records a CallEnumConstruct
// CallInfo. Returns (_, false) when the callee's base does not name an enum
// variant, so checkCall falls through to its other paths.
func (c *checker) checkEnumConstruct(n *ast.CallExpr) (Type, bool) {
	fa, ok := n.Callee.(*ast.FieldAccess)
	if !ok {
		return Invalid, false
	}
	tok, ei, ok := c.enumTypeOfBase(fa) // same-module `Enum.Variant`
	if !ok {
		// Cross-module `ns.Enum.Variant(arg)`: fa.X is itself `ns.Enum`.
		tok, ei, ok = c.qualifiedEnumOfBase(fa)
	}
	if !ok {
		return Invalid, false
	}
	if ei.Kind == EnumValue {
		// FR-022/SC-037: a value-enum variant is a folded constant, not a constructor.
		c.errf(fa.DotPos, "variant %q of value enum %q is not a constructor and cannot be called", fa.Field, ei.Name)
		c.typeArgs(n.Args)
		return Invalid, true
	}
	payload, hasPayload := ei.payload(fa.Field)
	if !variantExists(ei, fa.Field) {
		c.errf(fa.DotPos, "enum %q has no variant %q", ei.Name, fa.Field)
		c.typeArgs(n.Args)
		return Invalid, true
	}
	if !hasPayload {
		// SC-022/SC-024: any call form on a no-payload variant is an arity error.
		c.errf(fa.DotPos, "variant %s has no payload; write %s with no parentheses", fa.Field, fa.Field)
		c.typeArgs(n.Args)
		c.info.Types[n] = tok
		return tok, true
	}
	if len(n.Args) != 1 {
		// SC-023 (zero args) and SC-024 (extra args).
		c.errf(n.CalleePos, "variant %s has a payload; it expects 1 argument, got %d", fa.Field, len(n.Args))
		c.typeArgs(n.Args)
		c.info.Types[n] = tok
		return tok, true
	}
	at := c.checkExprExpecting(n.Args[0], payload)
	if at != Invalid && at != payload {
		c.errf(n.Args[0].Pos(), "variant %s payload has type %s, want %s", fa.Field, at, payload)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallEnumConstruct, EnumTok: tok, Variant: fa.Field, Args: n.Args, Result: tok}
	c.info.Types[n] = tok
	return tok, true
}

// checkBareTaggedVariant handles a bare (no-call) `Enum.Variant` reference where
// Enum is a tagged-union enum: a no-payload variant is a construction expression;
// a payload variant is an error -- the SC-020 FuncRef-exclusion (FR-017) when the
// reference is in a FuncRef-expecting position (`want` is a funcref), else the
// SC-030 "variant has a payload" diagnostic. `want` is the expected type threaded
// from checkFieldAccess so the two contexts are distinguished here.
func (c *checker) checkBareTaggedVariant(n *ast.FieldAccess, want Type) (Type, bool) {
	tok, ei, ok := c.enumTypeOfBase(n) // same-module `Enum.Variant`
	if !ok {
		// Cross-module bare `ns.Enum.Variant` (e.g. e.Expr.Unit): n.X is `ns.Enum`.
		// Task 6's tagged short-circuit makes checkQualifiedEnumVariantAccess return
		// (Invalid,false) for a cross-module tagged access, so checkFieldAccess falls
		// through to here; resolve the base the same way checkEnumConstruct does.
		tok, ei, ok = c.qualifiedEnumOfBase(n)
	}
	if !ok {
		return Invalid, false
	}
	if ei.Kind != EnumTagged {
		return Invalid, false
	}
	if !variantExists(ei, n.Field) {
		c.errf(n.DotPos, "enum %q has no variant %q", ei.Name, n.Field)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	_, hasPayload := ei.payload(n.Field)
	if hasPayload {
		// SC-020/FR-017: in a FuncRef-expecting position a payload variant looks
		// like a constructor function but is not a first-class function value.
		// This gate fires BEFORE the SC-030 branch (spec: FuncRef context governs).
		if want != Invalid && isFuncref(want) {
			c.errf(n.DotPos, "tagged-union variant %q cannot be used as a function reference; construct it with %s(...)", n.Field, n.Field)
			c.info.Types[n] = Invalid
			return Invalid, true
		}
		// SC-030: a bare payload-variant reference is neither construction nor fold.
		c.errf(n.DotPos, "variant %s has a payload; write %s(name) to construct it", n.Field, n.Field)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	// No-payload construction: record a CallEnumConstruct-equivalent so codegen
	// allocates a tag-only handle. A FieldAccess cannot live in Calls, so record
	// via Types + a dedicated map.
	c.info.Types[n] = tok
	c.markBareEnumConstruct(n, tok, n.Field)
	return tok, true
}

// markBareEnumConstruct records a bare no-payload variant construction
// (`Expr.Unit`) so codegen can allocate the tag-only handle.
func (c *checker) markBareEnumConstruct(n *ast.FieldAccess, tok Type, variant string) {
	c.info.BareEnumConstructs[n] = &BareEnumConstruct{EnumTok: tok, Variant: variant}
}
