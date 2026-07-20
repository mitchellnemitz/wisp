package types

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
)

// collectFoldConsts is the const collect-fold pass. It runs after Pass 2
// (struct fields resolved) and before Pass 4 (function bodies checked). It
// iterates ctx.prog.Consts in declaration order, folds each const-expression
// via the Task-2 evaluator, and records the result in info.ConstTable[name].
//
// Const references to earlier consts in the same module are resolved via
// c.constResolver (the hook wired by Task 2). Forward references and cycles
// are detected by an "in-progress" set: if the resolver is called for a name
// that is currently being folded (cycle) or has not yet been recorded
// (forward ref to a not-yet-folded const), an error is reported.
//
// After this pass, c.constResolver remains set so that body checking
// (checkParams default args, etc.) can also resolve top-level consts.
func (c *checker) collectFoldConsts(ctx *moduleCtx) {
	// Per-module cycle/forward-reference detection state, carried on ctx so the
	// resolver resolves against the module being checked in BOTH phases.
	ctx.foldInProgress = map[string]bool{}
	ctx.foldFailed = map[string]bool{}
	ctx.foldDeclIndex = map[string]*ast.ConstDecl{}
	for _, cd := range ctx.prog.Consts {
		// Keep the FIRST declaration of a name indexed. Duplicates are rejected
		// later via the seen-set, but overwriting here would make a fold-time
		// diagnostic (self/cycle/forward-reference) for the first declaration
		// point at the later duplicate's NamePos.
		if _, ok := ctx.foldDeclIndex[cd.Name]; !ok {
			ctx.foldDeclIndex[cd.Name] = cd
		}
	}

	c.installConstResolver()

	seen := map[string]bool{}
	for _, cd := range ctx.prog.Consts {
		if cd.Name == "_" {
			// blank const: fold for side-effects (errors) and check the annotation,
			// like a function-local `const _`, but create no binding.
			foldedType := c.checkConstExpr(cd.Value)
			annType := c.resolveType(cd.Type, cd.NamePos)
			if foldedType != Invalid && annType != Invalid && foldedType != annType {
				c.errf(cd.NamePos, "const %q: type mismatch: declared %s but initializer has type %s",
					cd.Name, annType, foldedType)
			}
			continue
		}

		// Validate the const name before folding/indexing it: reject reserved
		// names, duplicate top-level consts, and collisions with a top-level
		// function or struct. A rejected name is not indexed (no constTable /
		// topConsts / nav record) so later passes never act on it.
		if invalid := c.validateTopConstName(cd, ctx, seen); invalid {
			seen[cd.Name] = true
			continue
		}
		seen[cd.Name] = true

		ctx.foldingName = cd.Name
		ctx.foldInProgress[cd.Name] = true
		foldedType := c.checkConstExpr(cd.Value)
		delete(ctx.foldInProgress, cd.Name)
		ctx.foldingName = ""

		if foldedType == Invalid {
			// Error already recorded (by constResolver or checkConstExpr).
			ctx.foldFailed[cd.Name] = true
			continue
		}

		// Type-match: the folded type must match the declared annotation.
		annType := c.resolveType(cd.Type, cd.NamePos)
		if annType != Invalid && foldedType != annType {
			c.errf(cd.NamePos, "const %q: type mismatch: declared %s but initializer has type %s",
				cd.Name, annType, foldedType)
			ctx.foldFailed[cd.Name] = true
			continue
		}

		// Record in the module-local const table (scoped to this module).
		fv := c.info.FoldedValues[cd.Value]
		// Use the resolved annotation type (annType), not the inferred folded type.
		// They are equal for a well-typed const; if the annotation fails to resolve
		// the const stays Invalid rather than masquerading as an inferred binding.
		entry := &ConstEntry{Value: fv, Type: annType}
		ctx.constTable[cd.Name] = entry
		// Also mirror into the Info aggregate. For a single-module program (the
		// supported scope) this is exact; across modules it is informational only
		// (resolution uses the per-module table above).
		c.info.ConstTable[cd.Name] = entry

		// Create the nav record so Task 8 LSP can resolve top-level const
		// declarations via TopConstVars, parallel to ForInVars/CatchVars.
		// Top-level consts are never scoped to a function, so they have no
		// mangled shell name (codegen inlines them) and no curFunc Decls entry.
		topVar := &Var{
			Name:        cd.Name,
			Mangled:     "",
			Type:        annType,
			Pos:         cd.NamePos,
			IsConst:     true,
			FoldedValue: fv,
			Used:        true, // top-level consts are not subject to unused warnings
		}
		c.info.TopConstVars[cd] = topVar

		// Index by name (module-scoped) so foldConst/checkIdent can record
		// info.Uses for references to this module's consts.
		ctx.topConsts[cd.Name] = topVar

		// Cross-module export set (R1/R2): record an exported top-level const ONLY
		// after it has folded successfully and passed name validation, so a fold
		// failure never leaves a name marked exported. The export set is own-source
		// only: this records names declared `export const` IN THIS module. Reuses
		// moduleCtx.exported (the func+struct export set); a const name cannot
		// collide there because validateTopConstName rejects a const that shadows a
		// declared function or struct.
		if cd.Exported {
			ctx.exported[cd.Name] = true
		}
	}
}

// validateTopConstName reports whether the top-level const cd has an invalid
// name -- a reserved identifier or builtin/constant name, a duplicate of an
// earlier top-level const (tracked in seen), or a collision with a top-level
// function or struct. Each problem emits a compile error at cd.NamePos using
// the same wording as the local declareConst path. It returns true when the
// const should not be folded or indexed.
func (c *checker) validateTopConstName(cd *ast.ConstDecl, ctx *moduleCtx, seen map[string]bool) bool {
	invalid := false
	if isReservedIdent(cd.Name) {
		c.errf(cd.NamePos, "%q uses the reserved \"__\" namespace and cannot be a constant name", cd.Name)
		invalid = true
	} else if isReservedName(cd.Name) {
		c.errf(cd.NamePos, "%q is a reserved builtin or constant name and cannot be a constant name", cd.Name)
		invalid = true
	}
	if seen[cd.Name] {
		c.errf(cd.NamePos, "const %q is declared more than once", cd.Name)
		invalid = true
	}
	if _, ok := ctx.funcs[cd.Name]; ok {
		c.errf(cd.NamePos, "%q is a declared function and cannot be shadowed by a constant", cd.Name)
		invalid = true
	}
	if _, ok := ctx.structs[cd.Name]; ok {
		c.errf(cd.NamePos, "%q is a declared struct and cannot be shadowed by a constant", cd.Name)
		invalid = true
	}
	return invalid
}

// installConstResolver wires c.constResolver so const-to-const references work
// during folding and body checking. Resolution is keyed to c.cur, so a bare
// name resolves only against the CURRENT module's const table -- never another
// module's (export const is deferred). During body checking the scope stack is
// consulted first (via c.lookup), so a function-local const resolves before a
// same-named top-level const and resolution stays lexically scoped.
//
// When the resolver detects a cycle or forward reference it emits the precise
// diagnostic and returns (nil, Invalid, true). Returning true suppresses the
// generic "not a constant expression" fallback in foldConst (it returns the
// value/type as-is when ok == true, so (nil, Invalid, true) yields
// (Invalid, nil) without a second error).
func (c *checker) installConstResolver() {
	c.constResolver = func(name string) (interface{}, Type, bool) {
		// Local consts (body-checking phase) take priority and resolve through
		// the scope stack, so resolution respects lexical scope: a const declared
		// in an inner block is removed when that block's scope pops and cannot be
		// referenced from a const-expr outside it.
		if v := c.lookup(name); v != nil && v.IsConst {
			return v.FoldedValue, v.Type, true
		}
		ctx := c.cur
		if entry, ok := ctx.constTable[name]; ok {
			return entry.Value, entry.Type, true
		}
		// The cycle/forward-reference diagnostics below only apply while actively
		// folding a top-level const (foldingName set); they report against
		// foldDeclIndex[foldingName]. In any other phase -- body checking, or
		// folding a function-local const -- foldingName is "", so a name not in
		// constTable is simply unresolved: return false and let the use site
		// report the error. Dereferencing foldDeclIndex[""] here would panic
		// (e.g. a local const that references a top-level const which failed to
		// fold, like `const A = 10 / 0` referenced from a later const).
		if ctx.foldingName == "" {
			return nil, Invalid, false
		}
		if ctx.foldInProgress[name] {
			// Already being folded in this call stack: cycle.
			c.errf(ctx.foldDeclIndex[ctx.foldingName].NamePos,
				"const %q: cyclic reference involving %q", ctx.foldingName, name)
			return nil, Invalid, true
		}
		if ctx.foldFailed[name] {
			// Folded earlier but failed: this is part of a cycle that was broken
			// at the first failing const in the cycle.
			c.errf(ctx.foldDeclIndex[ctx.foldingName].NamePos,
				"const %q: cyclic reference involving %q", ctx.foldingName, name)
			return nil, Invalid, true
		}
		if cd, declared := ctx.foldDeclIndex[name]; declared {
			// Declared but not yet attempted: this may be a genuine forward
			// reference (Y declared after X) or one leg of a mutual cycle (P->Q
			// before Q is processed). Without a full graph pass we cannot
			// distinguish them, so use a label that is accurate for both cases.
			c.errf(cd.NamePos,
				"const %q: cyclic or forward reference to %q",
				ctx.foldingName, name)
			return nil, Invalid, true
		}
		return nil, Invalid, false
	}
}
