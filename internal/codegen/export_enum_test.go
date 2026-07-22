package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// TestExportedEnumVariantInlinesSameAsLocal proves the codegen claim behind
// this feature: a cross-module enum variant access folds to the SAME inlined
// integer literal a same-module access would, via the existing
// info.FoldedValues carrier in genExpr's FieldAccess case (codegen/expr.go).
// No codegen change was made for export enum; this test is the regression
// guard for that claim.
func TestExportedEnumVariantInlinesSameAsLocal(t *testing.T) {
	localSrc := `enum Color: int { Red, Green, Blue }
fn main() -> int {
    print(to_string(to_int(Color.Green)))
    return 0
}`
	localProg, err := parser.Parse(localSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse local: %v", err)
	}
	localLinked := &module.Linked{Modules: []*module.Module{{ID: 0, Prog: localProg, Namespaces: map[string]int{}}}}
	localInfo := types.CheckLinked(localLinked)
	if len(localInfo.Errors) != 0 {
		t.Fatalf("local check errors: %v", localInfo.Errors)
	}
	localOut, _, err := GenerateLinked(localLinked, localInfo)
	if err != nil {
		t.Fatalf("local GenerateLinked: %v", err)
	}

	// The leading blank line keeps `fn main` on line 2, matching localSrc's line
	// numbering (line 1 is `enum Color ...` there) -- so the generated `#
	// test.wisp:N` header comment lines up too, and the byte-identity comparison
	// below isolates the enum's effect instead of tripping on an unrelated
	// line-number difference between the two source strings.
	rootSrc := `
fn main() -> int {
    print(to_string(to_int(pal.Color.Green)))
    return 0
}`
	rootProg, err := parser.Parse(rootSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	libProg, err := parser.Parse(`export enum Color: int { Red, Green, Blue }`, "pal.wisp")
	if err != nil {
		t.Fatalf("parse lib: %v", err)
	}
	crossLinked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: rootProg, Namespaces: map[string]int{"pal": 1}},
		{ID: 1, Prog: libProg, Namespaces: map[string]int{}},
	}}
	crossInfo := types.CheckLinked(crossLinked)
	if len(crossInfo.Errors) != 0 {
		t.Fatalf("cross check errors: %v", crossInfo.Errors)
	}
	crossOut, _, err := GenerateLinked(crossLinked, crossInfo)
	if err != nil {
		t.Fatalf("cross GenerateLinked: %v", err)
	}

	// The strongest, non-fabricated proof of "no codegen change / no runtime
	// enum representation": the cross-module program and the equivalent
	// same-module program must generate BYTE-IDENTICAL shell. Both have `main`
	// as the only function (unsuffixed entrypoint), no other user symbols, and
	// the enum folds away entirely -- so if enums truly leave no runtime trace,
	// the two outputs cannot differ. This replaces the earlier weak
	// `Contains(out, "1")` (which passed on any "1" anywhere) and the
	// `"__wisp_enum"` needle (a name never grounded in codegen source).
	if string(localOut) != string(crossOut) {
		t.Fatalf("cross-module and same-module enum use must generate identical shell.\n--same-module--\n%s\n--cross-module--\n%s", localOut, crossOut)
	}
	// Sanity: the folded value 1 (Green) is actually present, so the identical
	// output is the real program, not two identical empty/degenerate scripts.
	if !strings.Contains(string(crossOut), "1") {
		t.Fatalf("expected the folded literal 1 (Green) in the generated output")
	}
}

// TestUnusedExportedEnumLeavesNoTrace proves SC-005: a program that includes a
// module exporting an enum it never references generates byte-identical shell to
// the same program with no such include at all.
func TestUnusedExportedEnumLeavesNoTrace(t *testing.T) {
	mainSrc := `fn main() -> int {
    print("hello")
    return 0
}`

	// A: standalone, no include.
	soloProg, err := parser.Parse(mainSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse solo: %v", err)
	}
	soloLinked := &module.Linked{Modules: []*module.Module{{ID: 0, Prog: soloProg, Namespaces: map[string]int{}}}}
	soloInfo := types.CheckLinked(soloLinked)
	if len(soloInfo.Errors) != 0 {
		t.Fatalf("solo check errors: %v", soloInfo.Errors)
	}
	soloOut, _, err := GenerateLinked(soloLinked, soloInfo)
	if err != nil {
		t.Fatalf("solo GenerateLinked: %v", err)
	}

	// B: same main, but importing a module that exports an enum it never uses.
	rootProg, err := parser.Parse(mainSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	libProg, err := parser.Parse(`export enum Color: int { Red, Green, Blue }`, "pal.wisp")
	if err != nil {
		t.Fatalf("parse lib: %v", err)
	}
	withLinked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: rootProg, Namespaces: map[string]int{"pal": 1}},
		{ID: 1, Prog: libProg, Namespaces: map[string]int{}},
	}}
	withInfo := types.CheckLinked(withLinked)
	if len(withInfo.Errors) != 0 {
		t.Fatalf("with-import check errors: %v", withInfo.Errors)
	}
	withOut, _, err := GenerateLinked(withLinked, withInfo)
	if err != nil {
		t.Fatalf("with-import GenerateLinked: %v", err)
	}

	if string(soloOut) != string(withOut) {
		t.Fatalf("unused exported enum must leave no trace.\n--no-import--\n%s\n--with-unused-export--\n%s", soloOut, withOut)
	}
}
