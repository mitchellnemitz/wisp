package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

func checkSrc(t *testing.T, src string) *Info {
	t.Helper()
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Check(prog)
}

func wantNoErr(t *testing.T, src string) {
	t.Helper()
	info := checkSrc(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors for %q, got: %v", src, info.Errors)
	}
}

func wantErr(t *testing.T, src, substr string) {
	t.Helper()
	info := checkSrc(t, src)
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, substr) {
			return
		}
	}
	t.Fatalf("expected an error containing %q for %q, got: %v", substr, src, info.Errors)
}

func mainWrap(body string) string {
	return "fn main() -> int {\n" + body + "\nreturn 0\n}\n"
}

// The bare sort/sort_by/find/any/all/slice/concat/sum/range/first/last/values/
// get/remove/merge/clamp/sign builtins are now removable (array/dict/math
// modules; get_or itself is fully removed, not merely moved -- see Task 6).
// Positive member-result coverage lives in core_arrays/core_dict/
// core_math_test.go; the negative type/domain checks moved to
// core_collections_neg_test.go. The checkSrc/wantNoErr/wantErr/mainWrap helpers
// above are retained: they are used by generics_test.go and optional_test.go.
