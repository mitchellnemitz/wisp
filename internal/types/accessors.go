package types

import (
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// TypeNames returns the wisp type-name set, sorted: the primitive type names
// plus the built-in error type. `error` is deliberately classified here as a
// TYPE (not a control keyword or a builtin), so editor grammars highlight it as
// a type. The values are the canonical spellings of the corresponding Type
// constants, so this set is the single source of truth and cannot drift from
// them.
func TypeNames() []string {
	out := []string{
		string(Int), string(Bool), string(String),
		string(Float), string(Void), string(ErrorType),
		string(RunResult), string(Process), "Optional",
	}
	sort.Strings(out)
	return out
}

// BuiltinNames returns every builtin-function name still reachable as a bare
// identifier, sorted: the "stays-flat" complement of `builtinSigs` after
// subtracting the removable set (the names moved to a module home; see
// RemovableBuiltins()). It is authoritative for tooling -- completions, hover,
// symbols, the formatter, `wisp doc`, and editor grammars must not advertise a
// removed name as a bare builtin. `builtinSigs` itself stays intact and
// unfiltered because it also backs the ns.member delegate/bridge path
// (checkCoreCall), which is unaffected by bare-surface removal. Note that the
// conversion/constructor builtins int/bool/string/float/error are also in
// TypeNames(); tooling subtracts TypeNames() from this set so those names are
// highlighted as types, not builtins (see the tooling plan's Conventions).
func BuiltinNames() []string {
	out := make([]string, 0, len(builtinSigs))
	for n := range builtinSigs {
		if isRemovableBuiltin(n) {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// CoreNamespaces returns the sorted names of the core modules (coreCatalog
// keys in internal/types/core_members.go), excluding any "__"-prefixed test
// sentinel namespace (the same exclusion crossNamespaceSuffix applies at
// core_members.go:363). Authoritative for tooling -- editor grammars reconcile
// their namespace-qualifier highlighting against this set.
func CoreNamespaces() []string {
	out := make([]string, 0, len(coreCatalog))
	for n := range coreCatalog {
		if strings.HasPrefix(n, "__") {
			continue
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// CoreMembers returns the sorted names of the callable (coreFunc) members of a
// core-module namespace, excluding coreType/coreConst members (so e.g.
// json.Value never leaks into json. completion). Returns nil if ns is not a
// registered core namespace or is a "__"-prefixed test sentinel.
func CoreMembers(ns string) []string {
	if strings.HasPrefix(ns, "__") {
		return nil
	}
	members, ok := coreCatalog[ns]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(members))
	for name, m := range members {
		if m.kind == coreFunc {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// coreMemberSig returns a core member's static signature and whether one is
// available. Per the coreMember field contract (core_members.go:46), m.sig is
// only meaningful when check == nil && !delegate; delegate entries are
// name-reservation stubs with no usable signature, and custom-checked members
// (e.g. json.decode) have no fixed static signature either.
func coreMemberSig(m coreMember) (builtinSig, bool) {
	return m.sig, m.check == nil && !m.delegate
}

// renderSigTail renders ONLY the parenthesized params and result tail of a
// builtin signature -- "(<p.name>: <types>[ = ...], ...) -> <result>" -- with
// no member name (the caller prepends "ns.member"). Mirrors funcSignature
// (lsp/server.go) but over builtinSig, using string(Type) and joinTypes so
// internal/types never imports internal/format.
func renderSigTail(s builtinSig) string {
	var b strings.Builder
	b.WriteByte('(')
	for i, p := range s.params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.name)
		b.WriteString(": ")
		b.WriteString(joinTypes(p.types))
		if p.hasDefault {
			b.WriteString(" = ...")
		}
	}
	b.WriteString(") -> ")
	b.WriteString(string(s.result))
	return b.String()
}

// CoreMemberHover returns the rendered hover detail for a core-module member:
// "(builtin) ns.member" name-only, or with a "(params) -> result" tail
// appended when a static signature is available. ok is false unless member is
// a coreFunc in coreCatalog[ns] (a coreType/coreConst member, like
// json.Value, is not callable and returns ok=false).
func CoreMemberHover(ns, member string) (string, bool) {
	members, ok := coreCatalog[ns]
	if !ok {
		return "", false
	}
	m, ok := members[member]
	if !ok || m.kind != coreFunc {
		return "", false
	}
	detail := "(builtin) " + ns + "." + member
	if sig, ok := coreMemberSig(m); ok {
		detail += renderSigTail(sig)
	}
	return detail, true
}

// MangleFunc returns the shell function name for a wisp function in module modid.
// It exposes the internal mangling so tooling and tests derive the name from the
// single source of truth instead of hardcoding the __wisp_f_m<modid>_<name> form.
func MangleFunc(modid int, name string) string { return mangleFunc(modid, name) }

// ReservedConstants returns the predefined reserved constant names (stdout,
// stderr), sorted. Authoritative for tooling's constant scope.
func ReservedConstants() []string {
	out := make([]string, 0, len(reservedConstants))
	for n := range reservedConstants {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ReservedNames returns the canonical, sorted, duplicate-free list of identifiers
// a user may NOT define: lexer keywords, the built-in type names (incl Optional,
// RunResult) plus Result, the reserved constants and sum constructors
// (Some/None/Ok/Err/stdout/stderr), and every builtin. ADDITIONALLY, any
// identifier beginning with "__" is reserved (a prefix rule, not enumerable here).
// The generic-bound words "comparable"/"numeric" are NOT reserved identifiers --
// they are context-sensitive keywords only in type-parameter bound position, so a
// user may define a function or variable named comparable/numeric -- and are
// deliberately excluded.
func ReservedNames() []string {
	set := map[string]struct{}{"Result": {}}
	for _, src := range [][]string{token.Keywords(), TypeNames(), ReservedConstants(), BuiltinNames()} {
		for _, n := range src {
			set[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
