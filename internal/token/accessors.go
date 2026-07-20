package token

import "sort"

// Keywords returns every reserved keyword word the lexer recognizes (the keys
// of the internal keyword table), sorted. It is the authoritative keyword set
// for tooling -- the LSP and the editor grammars derive their highlighting from
// it, so a keyword added to the lexer cannot silently drift out of the editors
// (the drift-guard test asserts equality against this accessor).
//
// This set INCLUDES the type-name words (int/bool/string/float/void) and error,
// which are also returned by types.TypeNames(); tooling partitions the scopes by
// subtracting types.TypeNames() so each identifier is highlighted under exactly
// one scope. See the tooling plan's Conventions.
func Keywords() []string {
	out := make([]string, 0, len(keywords))
	for w := range keywords {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}
