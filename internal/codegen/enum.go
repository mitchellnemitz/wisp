package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/types"
)

// genEnumConstruct lowers a tagged-union construction `Enum.Variant(arg)` to a
// fresh handle: tag = the variant-name identifier literal (injection-safe by the
// lexer charset, so it is emitted unquoted like Optional's "some"), value = the
// payload. FR-013: a scalar or value-enum payload is stored directly; a
// handle-typed payload stores its handle id (setHandleVar copies the id word).
func (g *gen) genEnumConstruct(ci *types.CallInfo) atom {
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom(ci.Variant))
	if len(ci.Args) == 1 {
		v := g.genExpr(ci.Args[0])
		g.setHandleVar(tagValueName(id), v)
	}
	return varAtom(id)
}

// genBareEnumConstruct lowers a bare no-payload construction `Enum.Unit`.
func (g *gen) genBareEnumConstruct(bc *types.BareEnumConstruct) atom {
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom(bc.Variant))
	return varAtom(id)
}
