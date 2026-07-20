package doc

import "testing"

// TestDocTypeAliasSignature: an alias renders `type Name = T` in its signature.
func TestDocTypeAliasSignature(t *testing.T) {
	r := func(src string) string {
		prog, comments := mustParse2(t, src)
		return Render("t.wisp", prog, comments)
	}
	assertContains(t, r("type Miles = int"), "type Miles = int")
	assertContains(t, r("type BinOp = fn(int, int) -> int"), "type BinOp = fn(int, int) -> int")
	assertContains(t, r("type Names = string[]"), "type Names = string[]")
}

// TestDocTypeAliasComment: a /// comment attaches to a type alias.
func TestDocTypeAliasComment(t *testing.T) {
	checkDoc(t, "/// Distance in miles.\ntype Miles = int", "Miles", "Distance in miles.")
}
