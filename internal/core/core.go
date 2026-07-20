// Package core holds the catalog of reserved bare import paths that resolve to
// synthetic wisp core modules (json, and later regex/env/fs/... -- Units 12-19)
// rather than fetched packages.
//
// This package is a leaf: it exports only the reserved namespace NAMES, which is
// all the module loader (internal/module) needs. The per-namespace member
// catalog (signatures, result types, codegen keys) lives in internal/types,
// because member checking needs the type checker's Type. Keeping the name list
// here avoids a module -> types import (which would be a cycle: types imports
// module).
package core

// namespaces is the set of reserved bare import paths. Adding a core module is a
// one-line addition here plus its member table in internal/types.
var namespaces = []string{"json", "regex", "env", "math", "fs", "process", "string", "dict", "array"}

// Namespaces returns the reserved bare import paths that resolve to synthetic
// core modules. The returned slice is a copy; callers may not mutate the table.
func Namespaces() []string {
	out := make([]string, len(namespaces))
	copy(out, namespaces)
	return out
}

// IsNamespace reports whether path is a reserved core-module bare import path.
func IsNamespace(path string) bool {
	for _, ns := range namespaces {
		if ns == path {
			return true
		}
	}
	return false
}
