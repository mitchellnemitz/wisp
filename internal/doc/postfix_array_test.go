package doc

import "testing"

// TestRenderArrayTypesPostfix: wisp doc renders array types in postfix `T[]`
// form (regression guard for the raw string(p.Type)/string(RetType) sites).
func TestRenderArrayTypesPostfix(t *testing.T) {
	r := func(src string) string {
		prog, comments := mustParse2(t, src)
		return Render("t.wisp", prog, comments)
	}
	// Function taking and returning arrays.
	assertContains(t, r("export fn f(xs: int[]) -> int[] { return xs }"), "fn f(xs: int[]) -> int[]")
	// Nested array param.
	assertContains(t, r("export fn g(m: int[][]) -> void { }"), "int[][]")
	// Struct field of array type.
	assertContains(t, r("export struct S { xs: string[] }"), "struct S { xs: string[] }")
}
