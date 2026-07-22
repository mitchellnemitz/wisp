package types

import (
	"strings"
	"testing"
)

// NOTE: do NOT define checkSrc here. The shared package helper
// `func checkSrc(t *testing.T, src string) *Info` already exists at
// internal/types/collections_test.go:10; a second definition in package types
// is a duplicate declaration that breaks `go build`/`go test ./internal/types`.
// This test reuses it directly.

func TestEnumPayloadRecursionResolves(t *testing.T) {
	cases := []struct {
		name, src, enum, variant, wantPayload string
		arrayElem                             string // non-empty => assert payload is an array of this elem, not an exact string
	}{
		{"direct self", `enum E { Rec(E), Base(int) } fn main() -> int { return 0 }`, "E", "Rec", "E", ""},
		{"through struct", `struct S { e: E } enum E { Wrap(S), Base(int) } fn main() -> int { return 0 }`, "E", "Wrap", "S", ""},
		{"through array", `enum E { Many(E[]), Base(int) } fn main() -> int { return 0 }`, "E", "Many", "", "E"},
		{"mutual A->B", `enum A { Ga(B), Ba(int) } enum B { Gb(A), Bb(int) } fn main() -> int { return 0 }`, "A", "Ga", "B", ""},
		{"mutual B->A", `enum A { Ga(B), Ba(int) } enum B { Gb(A), Bb(int) } fn main() -> int { return 0 }`, "B", "Gb", "A", ""},
	}
	for _, tc := range cases {
		info := checkSrc(t, tc.src)
		for _, d := range info.Errors {
			t.Errorf("%s: unexpected error: %s", tc.name, d.Msg)
		}
		// info.Enums is keyed by the internal token Name@modid, NOT the bare name
		// (internalEnumName, internal/types/linked.go:190; precedent
		// internal/types/enum_test.go:13). A single-module test program is modid 0.
		ei := info.Enums[string(internalEnumName(tc.enum, 0))]
		if ei == nil {
			t.Fatalf("%s: enum %s not registered; have %v", tc.name, tc.enum, info.Enums)
		}
		pt, ok := ei.payload(tc.variant)
		if !ok || pt == Invalid {
			// This is the assertion the empty pass-2 stub fails: the payload is
			// unresolved (Invalid) until checkEnumPayloads populates it.
			t.Errorf("%s: variant %s.%s payload unresolved (ok=%v, type=%q)", tc.name, tc.enum, tc.variant, ok, pt)
			continue
		}
		// A resolved payload Type is itself an internal token (e.g. E@0, S@0, or an
		// array wrapping E@0); assert it MENTIONS the expected type name rather than
		// hard-coding the token encoding (a bare `string(pt) == "E"` equality would
		// wrongly fail against the `E@0` token). The exact byte-level array/struct
		// form is pinned end-to-end by the Task 10 golden fixture.
		want := tc.wantPayload
		if tc.arrayElem != "" {
			want = tc.arrayElem
		}
		if !strings.Contains(string(pt), want) {
			t.Errorf("%s: %s.%s payload = %q, want it to mention %q", tc.name, tc.enum, tc.variant, pt, want)
		}
	}
}
