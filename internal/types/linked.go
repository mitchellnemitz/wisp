package types

import (
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// moduleCtx holds one module's symbol tables during checking.
type moduleCtx struct {
	id         int
	prog       *ast.Program
	namespaces map[string]int           // alias -> modid (from the loader)
	core       string                   // non-empty iff a synthetic core module (reserved namespace name)
	funcs      map[string]*ast.FuncDecl // this module's top-level functions
	exported   map[string]bool          // exported func+struct source names
	structs    map[string]*StructInfo   // by source name, this module
	enums      map[string]*EnumInfo     // by source name, this module (separate from structs, R2)
	aliases    map[string]*aliasInfo    // transparent type aliases, by source name, this module
	chain      []token.Position         // import/include chain from root

	// File-local const tables, scoped per module so a bare-name const reference
	// cannot leak across module boundaries and same-named consts in different
	// modules never collide (export const is a separate, deferred feature).
	constTable map[string]*ConstEntry // folded value+type, by source name
	topConsts  map[string]*Var        // declaration Var, by source name

	// Fold-time cycle/forward-reference detection state, set by collectFoldConsts
	// and read by the const resolver. Carried on the module so resolution during
	// body checking (Pass 4) is always keyed to the module being checked.
	foldInProgress map[string]bool
	foldFailed     map[string]bool
	foldDeclIndex  map[string]*ast.ConstDecl
	foldingName    string
}

// CheckLinked type-checks and resolves a linked multi-module program, producing
// one Info spanning all modules. A program is accepted iff len(Info.Errors) == 0.
func CheckLinked(linked *module.Linked) *Info {
	c := &checker{info: newInfo()}
	c.runLinked(linked)
	return c.info
}

func (c *checker) runLinked(linked *module.Linked) {
	// Pass 0: allocate one moduleCtx per module BEFORE any pass indexes c.modCtx.
	c.modCtx = make([]*moduleCtx, len(linked.Modules))
	for i, m := range linked.Modules {
		c.modCtx[i] = &moduleCtx{
			id: m.ID, prog: m.Prog, namespaces: m.Namespaces, core: m.Core,
			funcs: map[string]*ast.FuncDecl{}, exported: map[string]bool{},
			structs: map[string]*StructInfo{}, enums: map[string]*EnumInfo{}, chain: m.Chain,
			aliases:    map[string]*aliasInfo{},
			constTable: map[string]*ConstEntry{}, topConsts: map[string]*Var{},
		}
	}
	// Pass 1: collect structs (assigning internal tokens) and funcs; record exports.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		c.collectStructs(ctx)
		c.collectEnums(ctx)
		c.collectAliases(ctx)
		c.collectFuncs(ctx)
	}
	// Pass 2: resolve struct field types now every module's structs are known.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		c.checkStructFields(ctx)
	}
	// Pass 2.5: force-resolve every type alias, now that struct/enum names AND
	// struct field types are known (an alias RHS may be a generic instantiation
	// whose concrete StructInfo copies resolved base field types). Runs after
	// checkStructFields for that reason; the typeParams clear inside resolveAlias
	// makes resolution independent of any trigger context. This pass surfaces
	// errors for unused aliases (cycles, unknown RHS) too.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		c.resolveAliases(ctx)
	}
	// Pass 3: main belongs to the root only. c.cur = root so validateRootMain's
	// resolveType (on a struct/array-typed main param) uses the root's tables.
	c.cur = c.modCtx[0]
	c.checkMainRootOnly(linked)
	// Pass 3.5: collect and fold top-level consts for each module. This must
	// run after Pass 2 (struct field types resolved, so annotation resolveType
	// works) and before Pass 4 (body checking uses top-level const values in
	// default arguments and switch cases). Each module gets its own const table
	// scope (moduleCtx.constTable/topConsts); the const resolver keys off c.cur
	// so a bare-name reference resolves only within the module being checked.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		c.collectFoldConsts(ctx)
	}
	// Pass 4: check every function body in every module.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		for _, fn := range ctx.prog.Funcs {
			c.checkFunc(fn)
		}
	}
	// Pass 5: collect and check `test (...)` declarations. A `test` decl is only
	// legal in a `*_test.wisp` file; duplicate test names within one file are an
	// error (R3/AC13). Each body is type-checked as a `-> void` scope.
	for _, ctx := range c.modCtx {
		c.cur = ctx
		c.checkTests(ctx)
	}
}

// checkTests validates a module's `test (...)` declarations: it rejects them
// outside a `*_test.wisp` file, rejects duplicate test names within the file, and
// type-checks each body as a void scope.
func (c *checker) checkTests(ctx *moduleCtx) {
	testFile := isTestFile(ctx.prog.File)
	seen := map[string]bool{}
	for _, td := range ctx.prog.Tests {
		if !testFile {
			c.errf(td.KwPos, "`test` declarations are only allowed in a `*_test.wisp` file")
			continue
		}
		if seen[td.Name] {
			c.errf(td.NamePos, "test %q is declared more than once", td.Name)
			continue
		}
		seen[td.Name] = true
		c.checkTest(td)
	}
}

// checkTest type-checks one test body as a `-> void` function scope. It mirrors
// the body-checking portion of checkFunc, without registering a FuncInfo in
// info.Funcs (a test is not a callable function).
func (c *checker) checkTest(td *ast.TestDecl) {
	c.varCounter = 0
	c.loopDepth = 0
	c.tryDepth = 0
	c.loopTryDepth = nil
	c.typeParams = nil
	c.typeParamBounds = nil
	c.curRet = Void
	// The test body's FuncInfo carries its locals/spill-temp declarations so the
	// test-mode runner codegen can emit the body with proper `local` decls. A test
	// is not callable, so it has no mangled name and is NOT registered in
	// info.Funcs; it is recorded in info.Tests keyed by the TestDecl.
	fi := &FuncInfo{}
	c.curFunc = fi
	c.scopes = []scope{{}}
	c.checkBlock(td.Body)
	c.popScopeWarnUnused()
	c.curFunc = nil
	c.info.Tests[td] = fi
}

// checkMainRootOnly rejects `fn main` in any non-root module, then validates the
// root's single main signature.
func (c *checker) checkMainRootOnly(linked *module.Linked) {
	for _, m := range linked.Modules {
		if m.ID == 0 {
			continue
		}
		for _, fn := range m.Prog.Funcs {
			if fn.Name == "main" {
				// Set c.cur so the chain trailer names how this module was reached.
				c.cur = c.modCtx[m.ID]
				c.errf(fn.KwPos, "only the root file may define `fn main`; an imported or included module cannot")
				c.cur = c.modCtx[0]
			}
		}
	}
	c.validateRootMain(linked.Modules[0].Prog, isTestFile(linked.Modules[0].Prog.File))
}

// internalStructName is a struct's globally-unique type token Name@modid. `@` is
// illegal in a source identifier, so the token never collides with a user name or
// with the array/dict/funcref composite encodings, and never reaches the shell
// (struct handles are runtime-id keyed).
func internalStructName(name string, modid int) Type {
	return Type(name + "@" + strconv.Itoa(modid))
}

// internalEnumName is an enum's globally-unique type token Name@modid, the same
// shape as a struct token (disp strips the @<modid> suffix for diagnostics). An
// enum and a struct may not share a name (a located collision error in
// collectEnums), so the two registries' tokens never overlap in practice.
func internalEnumName(name string, modid int) Type {
	return Type(name + "@" + strconv.Itoa(modid))
}

// resolveNamedType resolves a bare or qualified named type to its internal token.
// A bare name is a local struct in the current module; a qualified `ns.Type` is an
// exported struct of the module bound to alias ns.
func (c *checker) resolveNamedType(s string, pos token.Position) Type {
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		ns, typ := s[:dot], s[dot+1:]
		modid, ok := c.cur.namespaces[ns]
		if !ok {
			c.errf(pos, "unknown namespace %q", ns)
			return Invalid
		}
		tctx := c.modCtx[modid]
		// A reserved core module resolves qualified types through the core catalog.
		if tctx.core != "" {
			if ct, ok := coreTypeMember(tctx.core, typ); ok {
				return ct
			}
			c.errf(pos, "module %q has no type %q", ns, typ)
			return Invalid
		}
		if _, ok := tctx.structs[typ]; !ok {
			c.errf(pos, "module %q has no type %q", ns, typ)
			return Invalid
		}
		if !tctx.exported[typ] {
			c.errf(pos, "type %q is not exported by %q", typ, ns)
			return Invalid
		}
		return internalStructName(typ, modid)
	}
	if _, ok := c.cur.structs[s]; ok {
		return internalStructName(s, c.cur.id)
	}
	if _, ok := c.cur.enums[s]; ok {
		return internalEnumName(s, c.cur.id)
	}
	c.errf(pos, "unknown type %q%s", s, suggestSuffix(s, c.typeNames()))
	return Invalid
}

// disp renders a Type for diagnostics, stripping each struct token's @<modid>
// suffix (a maximal digit run). Because `@` is illegal in source identifiers, the
// only `@`-then-digit run in any Type string is a struct-token suffix; an `@` not
// followed by a digit is left untouched (defensive; cannot occur).
func disp(t Type) string {
	s := string(t)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '@' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j - 1
			continue
		}
		if s[i] == '$' {
			continue // type-variable marker: render "$T" as "T"
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// chainTrailer renders an import/include chain as a diagnostic suffix.
func chainTrailer(chain []token.Position) string {
	var b strings.Builder
	b.WriteString(" (imported from ")
	for i, p := range chain {
		if i > 0 {
			b.WriteString(" -> ")
		}
		b.WriteString(p.String())
	}
	b.WriteString(")")
	return b.String()
}

// qualifiedNsTarget reports, for an expression that is a FieldAccess whose base
// is a bare identifier, the resolved target module id and whether the base is an
// in-scope namespace alias that is NOT shadowed by a local variable (precedence:
// a variable of the same name wins, keeping struct-funcref-field calls working).
// It recognizes the `ns.NAME` shape in both call position (`ns.fn(...)`, M8) and
// value/const position (`ns.NAME`, R3), so a cross-module reference is classified
// before the base ident is otherwise interpreted as a value (a namespace alias is
// not itself a value).
func (c *checker) qualifiedNsTarget(e ast.Expr) (field string, modid int, ok bool) {
	fa, isFA := e.(*ast.FieldAccess)
	if !isFA {
		return "", 0, false
	}
	id, isID := fa.X.(*ast.Ident)
	if !isID {
		return "", 0, false
	}
	if c.lookup(id.Name) != nil {
		return "", 0, false // a local variable shadows the namespace
	}
	m, isNS := c.cur.namespaces[id.Name]
	if !isNS {
		return "", 0, false
	}
	return fa.Field, m, true
}

// resolveGenericStructInst resolves a generic struct instantiation like "Box[int]"
// or "Pair[int,string]": it looks up the base struct, validates arity, resolves
// each type argument, and delegates to registerConcreteStructInst.
func (c *checker) resolveGenericStructInst(baseName, argsStr string, pos token.Position) Type {
	baseTok := c.resolveNamedType(baseName, pos)
	if baseTok == Invalid {
		return Invalid
	}
	baseSI, ok := c.info.Structs[string(baseTok)]
	if !ok {
		c.errf(pos, "internal: struct %q not in type table", baseName)
		return Invalid
	}
	if len(baseSI.TypeParams) == 0 {
		c.errf(pos, "struct %q is not generic", baseName)
		return Invalid
	}
	var rawArgs []Type
	if argsStr != "" {
		rawArgs = splitTopLevel(argsStr)
	}
	if len(rawArgs) != len(baseSI.TypeParams) {
		c.errf(pos, "generic struct %q requires %d type argument(s), got %d",
			baseName, len(baseSI.TypeParams), len(rawArgs))
		return Invalid
	}
	subst := map[string]Type{}
	for i, raw := range rawArgs {
		ct := c.resolveType(ast.TypeName(raw), pos)
		if ct == Invalid {
			return Invalid
		}
		subst[baseSI.TypeParams[i]] = ct
	}
	// LastIndexByte, not IndexByte: the current grammar only ever
	// produces one-dot qualifiers (ns.Name), so they're equivalent
	// today; Last keeps this correct if a future grammar change ever
	// admits multi-segment qualifiers, since it preserves the local
	// name regardless of how many dots precede it.
	unqualBase := baseName
	if dot := strings.LastIndexByte(baseName, '.'); dot >= 0 {
		unqualBase = baseName[dot+1:]
	}
	return c.registerConcreteStructInst(baseSI, unqualBase, subst)
}

// registerConcreteStructInst builds and registers a concrete StructInfo for the
// given base struct and substitution map. Returns the concrete token. If the
// instantiation is already registered, the existing token is returned unchanged.
func (c *checker) registerConcreteStructInst(baseSI *StructInfo, baseName string, subst map[string]Type) Type {
	argsEnc := ""
	for i, tp := range baseSI.TypeParams {
		if i > 0 {
			argsEnc += ","
		}
		argsEnc += string(subst[tp])
	}
	concreteTok := internalStructName(baseName+"["+argsEnc+"]", baseSI.ID)
	if _, exists := c.info.Structs[string(concreteTok)]; exists {
		return concreteTok
	}
	concreteSI := &StructInfo{
		Decl:   baseSI.Decl,
		Name:   baseName + "[" + argsEnc + "]",
		ID:     baseSI.ID,
		Fields: make([]StructFieldInfo, len(baseSI.Fields)),
		byName: map[string]Type{},
	}
	for i, f := range baseSI.Fields {
		ct := c.applySubst(f.Type, subst)
		concreteSI.Fields[i] = StructFieldInfo{Name: f.Name, Type: ct}
		concreteSI.byName[f.Name] = ct
	}
	c.info.Structs[string(concreteTok)] = concreteSI
	return concreteTok
}

// isInstantiationOf reports whether want is a parameterized instantiation of the
// same base struct as baseTok (same struct name and module ID, with type args).
func isInstantiationOf(want, baseTok Type) bool {
	ws := string(want)
	bs := string(baseTok)
	wat := strings.LastIndexByte(ws, '@')
	bat := strings.LastIndexByte(bs, '@')
	if wat < 0 || bat < 0 {
		return false
	}
	if ws[wat:] != bs[bat:] {
		return false // different module
	}
	wname := ws[:wat]
	bname := bs[:bat]
	bi := strings.IndexByte(wname, '[')
	if bi < 0 {
		return false // want has no type arguments
	}
	return wname[:bi] == bname
}

// isIdent reports whether s is a plain identifier: starts with a letter or
// underscore, followed by letters/digits/underscores. Struct names are always
// plain identifiers, so this rejects any non-struct token that happens to
// contain "[...]@..." -- most importantly a funcref whose return type is a
// generic-struct instantiation, e.g. "fn(int)->Box[T]@0", whose base-name
// slice up to the first "[" would otherwise be "fn(int)->Box".
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		isLetter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if i > 0 && !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// genericInstParts decomposes a generic-struct-instantiation token of the form
// "Name[arg1,arg2,...]@modid" into its base name, module suffix (including the
// "@"), and the raw (unsplit) argument-list text. Returns ok=false if t is not
// shaped like an instantiation token. modSuffix's digits-only requirement is
// grounded in internalStructName, which always builds the suffix via
// strconv.Itoa(modid) on an int.
func genericInstParts(t Type) (base, modSuffix, argsText string, ok bool) {
	s := string(t)
	at := strings.LastIndexByte(s, '@')
	if at < 0 {
		return "", "", "", false
	}
	modDigits := s[at+1:]
	if modDigits == "" {
		return "", "", "", false
	}
	for _, r := range modDigits {
		if r < '0' || r > '9' {
			return "", "", "", false
		}
	}
	name := s[:at]
	if !strings.HasSuffix(name, "]") {
		return "", "", "", false
	}
	bi := strings.IndexByte(name, '[')
	if bi < 0 {
		return "", "", "", false
	}
	base = name[:bi]
	if !isIdent(base) {
		return "", "", "", false
	}
	argsText = name[bi+1 : len(name)-1]
	if argsText == "" {
		return "", "", "", false
	}
	return base, s[at:], argsText, true
}
