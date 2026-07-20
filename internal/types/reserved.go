package types

import "strings"

// isReservedIdent reports whether name is in the compiler-reserved "__"
// namespace (spec section 5): any identifier beginning with two underscores is
// reserved for compiler internals and rejected in user code.
func isReservedIdent(name string) bool {
	return strings.HasPrefix(name, "__")
}
