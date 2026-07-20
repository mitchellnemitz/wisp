package types

import "testing"

// TestCoreCrossNamespaceSuggestion covers checkCoreCall's cross-namespace
// did-you-mean fallback: when a member exists verbatim on exactly one OTHER
// core namespace, the member-miss diagnostic should suggest it -- but only
// when no same-namespace fuzzy match already fired, and never when 2+ other
// namespaces share the member (ambiguous, matching suggestSuffix's own
// tie-breaks-to-nothing rule).
func TestCoreCrossNamespaceSuggestion(t *testing.T) {
	for _, c := range []struct {
		name string
		ns   string
		src  string
		want string
		bad  string // if non-empty, the error must NOT contain this substring
	}{
		{
			name: "unambiguous: dict.trim -> string.trim",
			ns:   "dict",
			src:  `fn main() -> int { dict.trim(); return 0 }`,
			want: `did you mean "string.trim"?`,
		},
		{
			name: "unambiguous: string.push -> array.push",
			ns:   "string",
			src:  `fn main() -> int { string.push(); return 0 }`,
			want: `did you mean "array.push"?`,
		},
		{
			name: "ambiguous 2-way tie (pre-existing): json.reverse silent",
			ns:   "json",
			src:  `fn main() -> int { json.reverse(); return 0 }`,
			want: `module "json" has no member "reverse"`,
			bad:  "did you mean",
		},
		{
			name: "same-namespace fuzzy wins over cross-namespace exact: json.set -> get",
			ns:   "json",
			src:  `fn main() -> int { json.set(); return 0 }`,
			want: `did you mean "get"?`,
			bad:  `env.set`,
		},
		{
			name: "ambiguous 2-way tie (new, introduced by Fix #1): dict.contains silent",
			ns:   "dict",
			src:  `fn main() -> int { dict.contains(); return 0 }`,
			want: `module "dict" has no member "contains"`,
			bad:  "did you mean",
		},
	} {
		info := checkNS(t, c.src, c.ns)
		if !hasErr(info, c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, errMsgs(info))
		}
		if c.bad != "" && hasErr(info, c.bad) {
			t.Errorf("%s: error must NOT contain %q, got %v", c.name, c.bad, errMsgs(info))
		}
	}
}

// TestCoreCrossNamespaceSentinelSkip proves crossNamespaceSuffix skips
// __-prefixed test-sentinel namespaces. It uses a member name that exists on
// NO production namespace (not "contains" -- that already ties 2-way between
// string/array in production after Fix #1, so it couldn't distinguish "skip
// works" from "skip is absent but a production tie happens to mask it").
// Without the skip, crossNamespaceSuffix would find exactly one match
// (__probe) and wrongly suggest "__probe.probe_sentinel_xyzzy", leaking the
// internal sentinel name into a user-facing diagnostic.
func TestCoreCrossNamespaceSentinelSkip(t *testing.T) {
	coreCatalog["__probe"] = map[string]coreMember{
		"probe_sentinel_xyzzy": {kind: coreFunc, builtin: "__probe_f", sig: coreSig0(Int)},
	}
	defer delete(coreCatalog, "__probe")

	info := checkNS(t, `fn main() -> int { dict.probe_sentinel_xyzzy(); return 0 }`, "dict")
	if !hasErr(info, `module "dict" has no member "probe_sentinel_xyzzy"`) {
		t.Fatalf("want member-miss error, got %v", errMsgs(info))
	}
	if hasErr(info, "did you mean") {
		t.Errorf("want no cross-namespace suggestion (sentinel must be skipped), got %v", errMsgs(info))
	}
}
