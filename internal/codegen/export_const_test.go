package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

func compileLinkedExport(t *testing.T, rootSrc, libSrc string, ns map[string]int) string {
	t.Helper()
	// Parse the root as "test.wisp" with a leading blank line so fn main lands at
	// line 2 -- matching the same-module case where a const declaration occupies
	// line 1 and fn main follows on line 2 in the same "test.wisp" file. The
	// byte-equality assertion in TestExportConstInlinesIntNoRuntimeSymbol relies on
	// these source comments being identical.
	rootProg, err := parser.Parse("\n"+rootSrc, "test.wisp")
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}
	libProg, err := parser.Parse(libSrc, "lib.wisp")
	if err != nil {
		t.Fatalf("parse lib: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: rootProg, Namespaces: ns},
		{ID: 1, Prog: libProg, Namespaces: map[string]int{}},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	out, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	return string(out)
}

func TestExportConstInlinesIntNoRuntimeSymbol(t *testing.T) {
	// AC15, strengthened to prove ZERO runtime symbol against the FULL script:
	// the cross-module reference must codegen byte-for-byte identically to the
	// SAME program written with a same-module const (PR #41's proven const-inline
	// path). Any stray assignment, expansion, or field-load for the const would
	// make the two scripts differ. The single-module `compile` helper lives in
	// codegen_test.go (same package).
	crossModule := compileLinkedExport(t,
		`fn main() -> int {`+"\n"+`  print(to_string(util.MAX))`+"\n"+`  return 0`+"\n"+`}`,
		`export const MAX: int = 3`, map[string]int{"util": 1})
	sameModule := string(compile(t,
		`const MAX: int = 3`+"\n"+
			`fn main() -> int {`+"\n"+`  print(to_string(MAX))`+"\n"+`  return 0`+"\n"+`}`))
	if crossModule != sameModule {
		t.Fatalf("cross-module const did not inline identically to same-module const.\ncross:\n%s\nsame:\n%s", crossModule, sameModule)
	}
	// And the script must actually contain the inlined literal (guards against the
	// degenerate case where both sides are equally wrong).
	if !strings.Contains(crossModule, "3") {
		t.Fatalf("folded value 3 not present in generated shell:\n%s", crossModule)
	}
	// Belt-and-suspenders source-symbol scan (subsumed by the byte-equality above,
	// kept as a fast, explicit signal).
	if strings.Contains(crossModule, "__wisp_s_") ||
		strings.Contains(crossModule, "MAX=") ||
		strings.Contains(crossModule, "$MAX") || strings.Contains(crossModule, "${MAX") {
		t.Errorf("qualified const leaked a runtime symbol:\n%s", crossModule)
	}
}

func TestExportConstInlinesBool(t *testing.T) {
	// bool has its own literal-emission path; exercise it across the boundary.
	root := `fn main() -> int {` + "\n" +
		`  let f: bool = util.FLAG` + "\n" +
		`  if (f) { print("yes") }` + "\n" +
		`  return 0` + "\n" +
		`}`
	lib := `export const FLAG: bool = true`
	s := compileLinkedExport(t, root, lib, map[string]int{"util": 1})
	if !strings.Contains(s, "true") {
		t.Fatalf("bool const not inlined as the true literal:\n%s", s)
	}
	if strings.Contains(s, "$FLAG") || strings.Contains(s, "FLAG=") {
		t.Errorf("bool const leaked a runtime symbol:\n%s", s)
	}
}

func TestExportConstInlinesStringInjectionSafe(t *testing.T) {
	root := `fn main() -> int {` + "\n" +
		`  print(util.UNSAFE)` + "\n" +
		`  return 0` + "\n" +
		`}`
	lib := `export const UNSAFE: string = "$(echo injected)"`
	s := compileLinkedExport(t, root, lib, map[string]int{"util": 1})
	// Single-quoted inert data, not a live subshell.
	if !strings.Contains(s, "'$(echo injected)'") {
		t.Fatalf("cross-module string const not single-quoted:\n%s", s)
	}
}

func TestExportConstInlinesFloat(t *testing.T) {
	root := `fn main() -> int {` + "\n" +
		`  print(to_string(util.PI))` + "\n" +
		`  return 0` + "\n" +
		`}`
	lib := `export const PI: float = 3.14`
	s := compileLinkedExport(t, root, lib, map[string]int{"util": 1})
	if !strings.Contains(s, "3.14") {
		t.Fatalf("float const 3.14 not inlined:\n%s", s)
	}
}

func TestExportConstInDefaultArgInlines(t *testing.T) {
	// AC3 codegen: a cross-module const default arg inlines its folded value at the
	// omitted-arg call site (PR #41 const_in_default mechanism), with no runtime
	// symbol for the const.
	root := `fn pause(secs: int = util.TIMEOUT) -> int { return secs }` + "\n" +
		`fn main() -> int {` + "\n" +
		`  print(to_string(pause()))` + "\n" +
		`  return 0` + "\n" +
		`}`
	lib := `export const TIMEOUT: int = 30`
	s := compileLinkedExport(t, root, lib, map[string]int{"util": 1})
	if !strings.Contains(s, "30") {
		t.Fatalf("default-arg const 30 not inlined:\n%s", s)
	}
	if strings.Contains(s, "$TIMEOUT") || strings.Contains(s, "TIMEOUT=") {
		t.Errorf("default-arg const leaked a runtime symbol:\n%s", s)
	}
}

func TestExportConstUnreferencedEmitsNoSymbol(t *testing.T) {
	// AC10, proven against the FULL script: an exported const that NO importer
	// references must leave NO trace. The generated shell with an unreferenced
	// export must be byte-for-byte identical to the same program whose lib has no
	// const at all. Any declaration, assignment, or value emission for the unused
	// const would make the two differ.
	root := `fn main() -> int {` + "\n" +
		`  print("hi")` + "\n" +
		`  return 0` + "\n" +
		`}`
	withUnused := compileLinkedExport(t, root, `export const UNUSED: int = 999`, map[string]int{"util": 1})
	withoutConst := compileLinkedExport(t, root, `export fn noop() -> int { return 0 }`, map[string]int{"util": 1})
	if withUnused != withoutConst {
		t.Fatalf("unreferenced exported const changed the output (it must emit nothing).\nwith UNUSED:\n%s\nwithout const:\n%s", withUnused, withoutConst)
	}
	// Explicit no-trace scan (subsumed by the byte-equality above).
	if strings.Contains(withUnused, "999") || strings.Contains(withUnused, "UNUSED") {
		t.Errorf("unreferenced exported const leaked a value or symbol:\n%s", withUnused)
	}
}
