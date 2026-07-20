package types

import "sort"

// This file computes the "modules-only" flat-surface removal sets mechanically
// from the two authoritative source tables (coreCatalog in core_members.go and
// builtinSigs in builtins.go). There is no hand-curated list of removed names
// anywhere: everything below is derived, so a new modularized builtin is swept
// into the removable set automatically.
//
// removable set  = { b : b is a coreFunc `builtin` key in coreCatalog } ∩ keys(builtinSigs)
// stays-flat set = keys(builtinSigs) - removable   (never materialized; use !removable[name])
//
// The builtinSigs table is NOT emptied by removal: the delegate/bridge path
// (checkCoreCall -> checkBuiltinNamed/checkBuiltinSig) still resolves ns.member
// calls by builtin key. Removal happens only in the bare-surface resolvers
// (checkNamedCall, expr.go value-reference) and in isReservedName's isBuiltin arm.

var removableBuiltinsMemo map[string]bool

// removableBuiltins returns the memoized set of flat builtin names removed from
// the bare surface: the intersection of coreCatalog coreFunc `builtin` keys with
// builtinSigs. json's 16 members are absent from builtinSigs (module-only from
// the start), so they are correctly excluded.
func removableBuiltins() map[string]bool {
	if removableBuiltinsMemo != nil {
		return removableBuiltinsMemo
	}
	m := map[string]bool{}
	for _, members := range coreCatalog {
		for _, mem := range members {
			if mem.kind != coreFunc || mem.builtin == "" {
				continue
			}
			if _, ok := builtinSigs[mem.builtin]; ok {
				m[mem.builtin] = true
			}
		}
	}
	removableBuiltinsMemo = m
	return m
}

// isRemovableBuiltin reports whether name is a flat builtin whose bare surface
// was removed (it is reachable only via its module home now).
func isRemovableBuiltin(name string) bool {
	return removableBuiltins()[name]
}

// removedHint returns the "ns.member" spelling to suggest for a removed flat
// name, and ok=false if name is not removable. A builtin key may in principle be
// exposed by more than one namespace member; the tie-break is deterministic:
// iterate namespaces in sorted order, then members in sorted order, and take the
// first coreFunc whose builtin key equals name. The returned member name is the
// catalog member name (e.g. reverse_string -> "string.reverse", env -> "env.get"),
// which may differ from the builtin key.
func removedHint(name string) (string, bool) {
	if !isRemovableBuiltin(name) {
		return "", false
	}
	nss := make([]string, 0, len(coreCatalog))
	for ns := range coreCatalog {
		nss = append(nss, ns)
	}
	sort.Strings(nss)
	for _, ns := range nss {
		members := coreCatalog[ns]
		memNames := make([]string, 0, len(members))
		for mn := range members {
			memNames = append(memNames, mn)
		}
		sort.Strings(memNames)
		for _, mn := range memNames {
			mem := members[mn]
			if mem.kind == coreFunc && mem.builtin == name {
				return ns + "." + mn, true
			}
		}
	}
	return "", false
}

// RemovableBuiltins returns the sorted list of removed flat builtin names.
// Exported so cross-package gates (residualcheck completeness, codegen tests)
// drive off the same derived inventory instead of a hand list.
func RemovableBuiltins() []string {
	m := removableBuiltins()
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// StaysFlatBuiltins returns the sorted list of builtin names that keep their bare
// flat surface: keys(builtinSigs) - removable.
func StaysFlatBuiltins() []string {
	rem := removableBuiltins()
	out := make([]string, 0, len(builtinSigs))
	for n := range builtinSigs {
		if !rem[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// RemovedHint returns the "ns.member" migration spelling for a removed flat name.
// Exported for tests asserting the module-hint diagnostic.
func RemovedHint(name string) (string, bool) { return removedHint(name) }
