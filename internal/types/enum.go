package types

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// collectEnums registers each enum declaration in module ctx into the SEPARATE
// enum registry (R2): info.Enums by internal token, ctx.enums by source name. It
// resolves C-style variant values (first unspecified = 0, each subsequent
// unspecified = previous+1; an explicit integer literal -- optionally negated --
// sets the value and reseeds the counter to it+1), then runs the distinct-value
// check AFTER full resolution so an auto-increment collision is caught. Duplicate
// variant names, a name colliding with a struct or reserved name, and a
// non-literal explicit value are located errors. The empty-enum case is rejected
// by the parser (T1). The caller sets c.cur = ctx first; collectStructs must have
// run already so the struct-name collision check sees this module's structs.
func (c *checker) collectEnums(ctx *moduleCtx) {
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

		ei := &EnumInfo{Decl: ed, Name: ed.Name, ID: ctx.id}
		seenName := map[string]bool{}
		next := int64(0)
		for _, v := range ed.Variants {
			if isReservedIdent(v.Name) {
				c.errf(v.NamePos, "%q uses the reserved \"__\" namespace and cannot be a variant name", v.Name)
			}
			if seenName[v.Name] {
				c.errf(v.NamePos, "variant %q is declared more than once in enum %q", v.Name, ed.Name)
				continue
			}
			seenName[v.Name] = true

			val := next
			if v.Value != nil {
				iv, ok := c.enumLiteralValue(v.Value)
				if !ok {
					// Diagnostic already emitted; skip this variant but keep the
					// counter advancing from the previous value so later variants are
					// still resolved deterministically.
					next++
					continue
				}
				val = iv
			}
			ei.Variants = append(ei.Variants, v.Name)
			ei.Values = append(ei.Values, val)
			next = val + 1
		}

		// Distinct-value check AFTER full resolution (incl. auto-increment), so a
		// collision produced by auto-increment (`enum E { A = 1, B = 0, C }` -> C=1
		// aliases A) is reported (R2/AC4). Located at the second occurrence.
		seenVal := map[int64]string{}
		for i, name := range ei.Variants {
			if first, ok := seenVal[ei.Values[i]]; ok {
				vp := variantPos(ed, name)
				c.errf(vp, "variant %q has duplicate value %d (already used by %q); enum values must be distinct", name, ei.Values[i], first)
				continue
			}
			seenVal[ei.Values[i]] = name
		}

		c.info.Enums[string(internalEnumName(ed.Name, ctx.id))] = ei
		ctx.enums[ed.Name] = ei
	}
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
	val, found := ei.value(n.Field)
	if !found {
		c.errf(n.DotPos, "enum %q has no variant %q", ei.Name, n.Field)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	c.info.Types[n] = tok
	c.info.FoldedValues[n] = val
	return tok, true
}

// checkIntEnumCall handles `to_int(e)` when e is an enum value (R4): it returns Int
// and records a builtin CallInfo so codegen lowers it (as identity -- the enum is
// already its int). The fixed `to_int` param table ({String, Float}) cannot express
// the enum operand, so this dedicated arm runs BEFORE the generic path. Returns
// (_, false) for a non-enum single argument so the call falls through to the
// generic table (to_int(string)/to_int(float)).
func (c *checker) checkIntEnumCall(n *ast.CallExpr) (Type, bool) {
	if len(n.Args) != 1 {
		return Invalid, false
	}
	at := c.checkExpr(n.Args[0])
	if !c.isEnumType(at) {
		return Invalid, false
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "to_int", Args: n.Args, Result: Int}
	return Int, true
}

// checkStringEnumCall rejects `to_string(e)` on an enum operand (R4): name rendering
// is deferred in v1, so leaking the underlying int is disallowed. Returns
// (Invalid, true) when the single argument is an enum (handled here), else
// (_, false) so the call falls through to the generic `to_string` table
// (int/float/bool/string), which the enum case cannot express.
func (c *checker) checkStringEnumCall(n *ast.CallExpr) (Type, bool) {
	if len(n.Args) != 1 {
		return Invalid, false
	}
	at := c.checkExpr(n.Args[0])
	if !c.isEnumType(at) {
		return Invalid, false
	}
	c.errf(n.Args[0].Pos(), "to_string() is not defined for enum %s (variant-name rendering is not yet supported); use to_int() for the value", disp(at))
	return Invalid, true
}

// isEnumType reports whether t names a declared enum. The separate enum registry
// makes this decidable; it is FALSE for structs/handles/primitives.
func (c *checker) isEnumType(t Type) bool {
	_, ok := c.info.Enums[string(t)]
	return ok
}
