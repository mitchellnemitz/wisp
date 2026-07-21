package types_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// TestBuiltinsAreSnakeCase enforces the standard library's permanent naming
// convention: every builtin is snake_case (lowercase, digits, underscores). This
// fails CI if a camelCase or PascalCase builtin is ever added. See AGENTS.md and
// www/src/content/docs/guide/stdlib.md.
func TestBuiltinsAreSnakeCase(t *testing.T) {
	snake := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, name := range types.BuiltinNames() {
		if !snake.MatchString(name) {
			t.Errorf("builtin %q is not snake_case; the stdlib is snake_case only (no camelCase/PascalCase)", name)
		}
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestTypeNames(t *testing.T) {
	tn := types.TypeNames()
	if !sort.StringsAreSorted(tn) {
		t.Errorf("TypeNames() not sorted: %v", tn)
	}
	for _, want := range []string{"int", "bool", "string", "float", "void", "error", "Optional", "RunResult", "Process"} {
		if !contains(tn, want) {
			t.Errorf("TypeNames() missing %q: %v", want, tn)
		}
	}
	if len(tn) != 9 {
		t.Errorf("TypeNames() = %v, want exactly 9 names", tn)
	}
}

func TestBuiltinAndConstAccessors(t *testing.T) {
	bn := types.BuiltinNames()
	if !sort.StringsAreSorted(bn) {
		t.Errorf("BuiltinNames() not sorted: %v", bn)
	}
	// Stays-flat builtins remain in BuiltinNames().
	for _, want := range []string{"print", "length", "to_int", "to_string", "error", "set_stdin", "parse_args", "assert", "unwrap"} {
		if !contains(bn, want) {
			t.Errorf("BuiltinNames() missing stays-flat name %q", want)
		}
	}
	// Removable builtins (those with a module home) are no longer bare builtins.
	for _, absent := range []string{"map", "filter", "split", "join", "abs", "reduce", "get", "sort", "read_file", "env"} {
		if contains(bn, absent) {
			t.Errorf("BuiltinNames() must NOT contain removable name %q", absent)
		}
	}
	rc := types.ReservedConstants()
	if !sort.StringsAreSorted(rc) {
		t.Errorf("ReservedConstants() not sorted: %v", rc)
	}
	if len(rc) != 6 || !contains(rc, "stdout") || !contains(rc, "stderr") || !contains(rc, "Some") || !contains(rc, "None") || !contains(rc, "Ok") || !contains(rc, "Err") {
		t.Errorf("ReservedConstants() = %v, want [Err None Ok Some stderr stdout]", rc)
	}
}

// TestReservedNames_ContainsAllSources asserts (AC1/AC2) that ReservedNames()
// is sorted, deduplicated, includes all entries from each source, and that
// the well-known names int/error/string/float appear exactly once.
func TestReservedNames_ContainsAllSources(t *testing.T) {
	rn := types.ReservedNames()

	if !sort.StringsAreSorted(rn) {
		t.Errorf("ReservedNames() not sorted: %v", rn)
	}

	// No duplicates: map length must equal slice length.
	seen := map[string]struct{}{}
	for _, n := range rn {
		seen[n] = struct{}{}
	}
	if len(seen) != len(rn) {
		t.Errorf("ReservedNames() has duplicates: len=%d, unique=%d", len(rn), len(seen))
	}

	// Every entry from each source must be present.
	for _, src := range [][]string{token.Keywords(), types.TypeNames(), types.ReservedConstants(), types.BuiltinNames()} {
		for _, n := range src {
			if !contains(rn, n) {
				t.Errorf("ReservedNames() missing %q (from source)", n)
			}
		}
	}
	if !contains(rn, "Result") {
		t.Errorf("ReservedNames() missing %q", "Result")
	}

	// Spot-check well-known names present exactly once.
	for _, name := range []string{"int", "error", "string", "float"} {
		count := 0
		for _, n := range rn {
			if n == name {
				count++
			}
		}
		if count != 1 {
			t.Errorf("ReservedNames(): %q appears %d times, want exactly 1", name, count)
		}
	}
}

// TestReservedNames_ExcludesContextSensitiveWords asserts (AC3) that
// comparable, numeric, and any __-prefixed name are absent from ReservedNames().
func TestReservedNames_ExcludesContextSensitiveWords(t *testing.T) {
	rn := types.ReservedNames()
	for _, absent := range []string{"comparable", "numeric"} {
		if contains(rn, absent) {
			t.Errorf("ReservedNames() must NOT contain %q (context-sensitive bound word)", absent)
		}
	}
	for _, n := range rn {
		if strings.HasPrefix(n, "__") {
			t.Errorf("ReservedNames() must NOT contain __ -prefixed name %q (prefix rule, not enumerable)", n)
		}
	}
}

// TestCoreNamespaces verifies (spec "Authoritative source (compiler accessor)")
// that CoreNamespaces() returns exactly the 9 core modules, sorted, and
// excludes any "__"-prefixed test sentinel namespace -- the editor-side tests
// cannot observe the "__" exclusion, so the accessor owns that proof.
func TestCoreNamespaces(t *testing.T) {
	got := types.CoreNamespaces()
	want := []string{"array", "dict", "env", "fs", "json", "math", "process", "regex", "string"}
	if len(got) != len(want) {
		t.Fatalf("CoreNamespaces() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CoreNamespaces() = %v, want %v", got, want)
		}
	}
	for _, n := range got {
		if strings.HasPrefix(n, "__") {
			t.Errorf("CoreNamespaces() must NOT contain __ -prefixed sentinel %q", n)
		}
	}
}

// TestCoreMembers verifies CoreMembers returns callable members only, sorted,
// and empty for a non-core or "__"-prefixed namespace.
func TestCoreMembers(t *testing.T) {
	sm := types.CoreMembers("string")
	if !sort.StringsAreSorted(sm) {
		t.Errorf("CoreMembers(string) not sorted: %v", sm)
	}
	for _, want := range []string{"trim", "split", "join"} {
		if !contains(sm, want) {
			t.Errorf("CoreMembers(string) missing %q: %v", want, sm)
		}
	}
	jm := types.CoreMembers("json")
	if contains(jm, "Value") {
		t.Errorf("CoreMembers(json) must NOT contain coreType member %q: %v", "Value", jm)
	}
	if len(jm) == 0 {
		t.Errorf("CoreMembers(json) is empty, want at least the coreFunc members")
	}
	if got := types.CoreMembers("bogus"); len(got) != 0 {
		t.Errorf("CoreMembers(bogus) = %v, want empty", got)
	}
	if got := types.CoreMembers("__x"); len(got) != 0 {
		t.Errorf("CoreMembers(__x) = %v, want empty", got)
	}
}

// TestCoreMemberHover pins the exact hover literals from the spec: a
// fixed-signature member gets a rendered tail, a delegate member and a
// custom-checked member are name-only, and unknown/non-callable members are
// ok=false.
func TestCoreMemberHover(t *testing.T) {
	if got, ok := types.CoreMemberHover("string", "trim"); !ok || got != "(builtin) string.trim(a: string) -> string" {
		t.Errorf("CoreMemberHover(string, trim) = (%q, %v), want (%q, true)", got, ok, "(builtin) string.trim(a: string) -> string")
	}
	if got, ok := types.CoreMemberHover("array", "map"); !ok || got != "(builtin) array.map" {
		t.Errorf("CoreMemberHover(array, map) = (%q, %v), want (%q, true)", got, ok, "(builtin) array.map")
	}
	if got, ok := types.CoreMemberHover("json", "decode"); !ok || got != "(builtin) json.decode" {
		t.Errorf("CoreMemberHover(json, decode) = (%q, %v), want (%q, true)", got, ok, "(builtin) json.decode")
	}
	if _, ok := types.CoreMemberHover("string", "nope"); ok {
		t.Errorf("CoreMemberHover(string, nope) ok = true, want false")
	}
	if _, ok := types.CoreMemberHover("json", "Value"); ok {
		t.Errorf("CoreMemberHover(json, Value) ok = true, want false (coreType, not callable)")
	}
}

// TestCoreMemberCoverageAllNamespaces iterates every CoreNamespaces() entry
// and asserts CoreMembers is non-empty and CoreMemberHover resolves the first
// member with the "(builtin) <ns>." prefix, so no namespace is silently
// unwired.
func TestCoreMemberCoverageAllNamespaces(t *testing.T) {
	for _, ns := range types.CoreNamespaces() {
		members := types.CoreMembers(ns)
		if len(members) == 0 {
			t.Fatalf("CoreMembers(%q) is empty", ns)
		}
		first := members[0]
		detail, ok := types.CoreMemberHover(ns, first)
		if !ok {
			t.Fatalf("CoreMemberHover(%q, %q) ok = false, want true", ns, first)
		}
		want := "(builtin) " + ns + "."
		if !strings.HasPrefix(detail, want) {
			t.Errorf("CoreMemberHover(%q, %q) = %q, want prefix %q", ns, first, detail, want)
		}
	}
}

// TestScopePartitionDisjoint verifies the tooling scope partition (tooling plan
// Conventions): control-keyword = Keywords() \ TypeNames(); type = TypeNames();
// builtin = BuiltinNames() \ TypeNames(); const = ReservedConstants(). The four
// derived scopes must be PAIRWISE DISJOINT so every identifier is highlighted
// under exactly one scope. This is the invariant the editor grammars rely on;
// it is NOT a subset claim (void is in TypeNames() but not in BuiltinNames()).
func TestScopePartitionDisjoint(t *testing.T) {
	toSet := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	typeSet := toSet(types.TypeNames())
	minusTypes := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			if !typeSet[x] {
				m[x] = true
			}
		}
		return m
	}

	control := minusTypes(token.Keywords())
	types_ := typeSet
	builtin := minusTypes(types.BuiltinNames())
	consts := toSet(types.ReservedConstants())

	scopes := map[string]map[string]bool{
		"control": control, "type": types_, "builtin": builtin, "const": consts,
	}
	for an, a := range scopes {
		for bn, b := range scopes {
			if an >= bn {
				continue
			}
			for w := range a {
				if b[w] {
					t.Errorf("scope %q and %q both contain %q (partition not disjoint)", an, bn, w)
				}
			}
		}
	}

	// error must land in the type scope and nowhere else.
	if !types_["error"] {
		t.Error("type scope missing error")
	}
	if control["error"] || builtin["error"] {
		t.Error("error leaked into control/builtin scope")
	}
	// true/false are control keywords, not types.
	if !control["true"] || !control["false"] {
		t.Error("true/false should be in the control-keyword scope")
	}
}
