package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// TestBuiltinRef_Consistency is the bidirectional consistency gate binding the
// three sources of truth for universal builtin funcrefs:
//   - the checker allowlist (types.GeneratableBuiltinFuncrefs),
//   - the runtime wrapper spec table (runtime.BuiltinWrapperIDs), and
//   - the runtime prelude registry (runtime.IDs), where a wrapper snippet's id
//     equals its funcref mangled name "__wisp_builtin_<name>".
//
// Every generatable builtin must have exactly one wrapper spec and one prelude
// snippet, and every wrapper spec / prefixed prelude id must map back to a
// generatable builtin. A one-way check would pass while a stale wrapper or a
// missing snippet lingered, so both directions are required.
func TestBuiltinRef_Consistency(t *testing.T) {
	const prefix = "__wisp_builtin_"

	generatable := types.GeneratableBuiltinFuncrefs()

	// Overloaded builtins (abs/min/max/clamp/sign) have no single funcref type;
	// each arm mints its own wrapper id ("__wisp_builtin_<name>_<arm>") that is
	// NOT of the form prefix+name for a generatable name, so they are checked
	// against this separate allowlist instead of `generatable`.
	overloadedIDs := types.OverloadedFuncrefWrapperIDs()
	overloadedSet := make(map[string]bool, len(overloadedIDs))
	for _, id := range overloadedIDs {
		overloadedSet[id] = true
	}

	// Generic higher-order builtins (map/filter/each/reduce/sort_by/find/any/
	// all/count_where/and_then/or_else/map_err) mint one wrapper id per
	// container-shape axis, same treatment as an overload arm: NOT of the form
	// prefix+name for a generatable name, so checked against this separate
	// allowlist too.
	genericIDs := types.GenericFuncrefWrapperIDs()
	for _, id := range genericIDs {
		overloadedSet[id] = true
	}

	wrapperIDs := runtime.BuiltinWrapperIDs()
	wrapperSet := make(map[string]bool, len(wrapperIDs))
	for _, id := range wrapperIDs {
		if wrapperSet[id] {
			t.Errorf("duplicate wrapper id %q in runtime.BuiltinWrapperIDs()", id)
		}
		wrapperSet[id] = true
	}

	registrySet := make(map[string]bool)
	for _, id := range runtime.IDs() {
		if strings.HasPrefix(id, prefix) {
			registrySet[id] = true
		}
	}

	// Forward: each generatable builtin has a wrapper spec AND a prelude snippet.
	for name := range generatable {
		mangled := prefix + name
		if !wrapperSet[mangled] {
			t.Errorf("generatable builtin %q has no runtime wrapper spec for %q", name, mangled)
		}
		if !registrySet[mangled] {
			t.Errorf("generatable builtin %q has no prelude snippet for %q", name, mangled)
		}
		if !runtime.IsBuiltinWrapperID(mangled) {
			t.Errorf("runtime.IsBuiltinWrapperID(%q) = false, want true", mangled)
		}
	}

	// Forward: each overload arm has a wrapper spec AND a prelude snippet.
	for _, mangled := range overloadedIDs {
		if !wrapperSet[mangled] {
			t.Errorf("overloaded arm %q has no runtime wrapper spec", mangled)
		}
		if !registrySet[mangled] {
			t.Errorf("overloaded arm %q has no prelude snippet", mangled)
		}
		if !runtime.IsBuiltinWrapperID(mangled) {
			t.Errorf("runtime.IsBuiltinWrapperID(%q) = false, want true", mangled)
		}
	}

	// Forward: each generic axis has a wrapper spec AND a prelude snippet.
	for _, mangled := range genericIDs {
		if !wrapperSet[mangled] {
			t.Errorf("generic axis %q has no runtime wrapper spec", mangled)
		}
		if !registrySet[mangled] {
			t.Errorf("generic axis %q has no prelude snippet", mangled)
		}
		if !runtime.IsBuiltinWrapperID(mangled) {
			t.Errorf("runtime.IsBuiltinWrapperID(%q) = false, want true", mangled)
		}
	}

	// Reverse: each wrapper spec maps back to a generatable builtin OR an
	// overload arm.
	for id := range wrapperSet {
		if overloadedSet[id] {
			continue
		}
		if !strings.HasPrefix(id, prefix) {
			t.Errorf("wrapper id %q lacks the %q prefix", id, prefix)
			continue
		}
		if name := strings.TrimPrefix(id, prefix); !generatable[name] {
			t.Errorf("wrapper id %q has no corresponding generatable builtin %q", id, name)
		}
	}

	// Reverse: each prefixed prelude snippet maps back to a generatable builtin
	// OR an overload arm.
	for id := range registrySet {
		if overloadedSet[id] {
			continue
		}
		if name := strings.TrimPrefix(id, prefix); !generatable[name] {
			t.Errorf("prelude snippet %q has no corresponding generatable builtin %q", id, name)
		}
	}
}
