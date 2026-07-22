package types

import "github.com/mitchellnemitz/wisp/internal/ast"

// Transparent (structural) type aliases: `type Name = T`. An alias is a pure
// synonym for the type annotation T. It is resolved to T's underlying resolved
// Type inside resolveType (the single chokepoint that decodes every composite
// and resolves every named type), so the alias name never reaches type-equality,
// codegen, source maps, or the runtime. Aliases are module-local, like enums.

// aliasResolveState tracks a type alias through resolution so cycles are caught
// (a re-entry into an inProgress alias) instead of recursing forever.
type aliasResolveState int

const (
	aliasPending aliasResolveState = iota
	aliasInProgress
	aliasDone
)

// aliasInfo is one module-local type alias: its declaration, its resolution
// state, and (once done) the resolved underlying Type. On a cycle or an
// unresolvable RHS the resolved Type is Invalid.
type aliasInfo struct {
	decl     *ast.TypeAliasDecl
	state    aliasResolveState
	resolved Type
}

// collectAliases registers each `type Name = T` declaration in module ctx. It
// rejects a blank name, a reserved name, a primitive type name, and a name that
// collides with a struct, an enum, or another alias in the module. RHS resolution
// happens later (resolveAliases). The caller sets c.cur = ctx first; collectStructs
// and collectEnumNames must have run so the collision checks see this module's types.
func (c *checker) collectAliases(ctx *moduleCtx) {
	for _, ad := range ctx.prog.Aliases {
		if ad.Name == "_" {
			c.errf(ad.NamePos, "type alias name cannot be blank")
			continue
		}
		if isReservedIdent(ad.Name) {
			c.errf(ad.NamePos, "%q uses the reserved \"__\" namespace and cannot be a type-alias name", ad.Name)
		}
		if isReservedName(ad.Name) {
			c.errf(ad.NamePos, "%q is a reserved builtin or constant name and cannot be a type-alias name", ad.Name)
		}
		if isPrimitiveTypeName(ad.Name) {
			c.errf(ad.NamePos, "%q is a built-in type name and cannot be a type-alias name", ad.Name)
		}
		if _, dup := ctx.aliases[ad.Name]; dup {
			c.errf(ad.NamePos, "type alias %q is declared more than once", ad.Name)
			continue
		}
		if _, clash := ctx.structs[ad.Name]; clash {
			c.errf(ad.NamePos, "%q is already declared as a struct; a type alias and a struct cannot share a name", ad.Name)
			continue
		}
		if _, clash := ctx.enums[ad.Name]; clash {
			c.errf(ad.NamePos, "%q is already declared as an enum; a type alias and an enum cannot share a name", ad.Name)
			continue
		}
		ctx.aliases[ad.Name] = &aliasInfo{decl: ad, state: aliasPending}
	}
}

// resolveAliases forces resolution of every alias in module ctx (including
// unused ones), so cycle and unknown-RHS errors surface regardless of use. The
// caller sets c.cur = ctx first. Runs after checkStructFields so an alias RHS
// that is a generic instantiation copies resolved base field types.
func (c *checker) resolveAliases(ctx *moduleCtx) {
	// Iterate the source-ordered decl slice (not the map) so diagnostics for
	// multiple failing aliases are emitted deterministically, matching every other
	// collection pass. A duplicate/blank alias absent from ctx.aliases is skipped by
	// resolveAlias's nil guard.
	for _, ad := range ctx.prog.Aliases {
		c.resolveAlias(ctx, ad.Name)
	}
}

// resolveAlias returns the resolved underlying Type of the named alias in ctx,
// resolving it on demand. It is the fixpoint used both by the forcing pass and
// by the resolveType hook. A cycle (re-entry into an inProgress alias) is a clean
// located error and resolves to Invalid; there is no unbounded recursion.
//
// resolveType consults c.typeParams before the alias hook, so this method clears
// c.typeParams around the RHS resolution: an alias is top-level and has no type
// parameters in scope, so a bare type-parameter name in the RHS must resolve as
// an unknown named type, not as the type variable of whatever generic context
// happened to trigger the resolution.
func (c *checker) resolveAlias(ctx *moduleCtx, name string) Type {
	ai := ctx.aliases[name]
	if ai == nil {
		return Invalid
	}
	switch ai.state {
	case aliasDone:
		return ai.resolved
	case aliasInProgress:
		c.errf(ai.decl.NamePos, "type alias cycle through %q", name)
		ai.resolved = Invalid
		ai.state = aliasDone
		return ai.resolved
	}
	ai.state = aliasInProgress
	saved := c.typeParams
	c.typeParams = nil
	ai.resolved = c.resolveType(ai.decl.Type, ai.decl.TypePos)
	c.typeParams = saved
	ai.state = aliasDone
	return ai.resolved
}
