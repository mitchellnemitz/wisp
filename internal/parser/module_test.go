package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func TestParseExportModifier(t *testing.T) {
	prog := parseOK(t, "export fn f() -> int { return 0 }\nexport struct P { x: int }\nfn main() -> int { return 0 }")
	var f, main *ast.FuncDecl
	for _, fn := range prog.Funcs {
		switch fn.Name {
		case "f":
			f = fn
		case "main":
			main = fn
		}
	}
	if f == nil || !f.Exported {
		t.Errorf("export fn f not marked Exported: %+v", f)
	}
	if f.ExportPos.Line == 0 {
		t.Errorf("export fn f missing ExportPos")
	}
	if main == nil || main.Exported {
		t.Errorf("fn main should not be Exported: %+v", main)
	}
	if len(prog.Structs) != 1 || !prog.Structs[0].Exported {
		t.Errorf("export struct P not marked Exported: %+v", prog.Structs)
	}
}

func TestParseExportOnNonDecl(t *testing.T) {
	for _, src := range []string{
		"export let x: int = 1\nfn main() -> int { return 0 }",
		"export 1\nfn main() -> int { return 0 }",
		"export\nfn main() -> int { return 0 }",
	} {
		parseErr(t, src)
	}
}

func TestParseImportDecl(t *testing.T) {
	prog := parseOK(t, `import "owner/repo"`+"\n"+`import "a/b" as c`+"\nfn main() -> int { return 0 }")
	if len(prog.Imports) != 2 {
		t.Fatalf("want 2 imports, got %d", len(prog.Imports))
	}
	if prog.Imports[0].Path != "owner/repo" || prog.Imports[0].Alias != "" {
		t.Errorf("import 0 = %+v", prog.Imports[0])
	}
	if prog.Imports[1].Path != "a/b" || prog.Imports[1].Alias != "c" {
		t.Errorf("import 1 = %+v", prog.Imports[1])
	}
	if prog.Imports[1].PathPos.Line == 0 || prog.Imports[1].AliasPos.Line == 0 {
		t.Errorf("import 1 missing positions: %+v", prog.Imports[1])
	}
}

func TestParseIncludeDecl(t *testing.T) {
	prog := parseOK(t, `include "./x.wisp"`+"\n"+`include "./lib/util.wisp" as util`+"\nfn main() -> int { return 0 }")
	if len(prog.Includes) != 2 {
		t.Fatalf("want 2 includes, got %d", len(prog.Includes))
	}
	if prog.Includes[0].Path != "./x.wisp" || prog.Includes[0].Alias != "" {
		t.Errorf("include 0 = %+v", prog.Includes[0])
	}
	if prog.Includes[1].Path != "./lib/util.wisp" || prog.Includes[1].Alias != "util" {
		t.Errorf("include 1 = %+v", prog.Includes[1])
	}
}

func TestParseImportInterpolatedPathErrors(t *testing.T) {
	parseErr(t, `import "a/${b}"`+"\nfn main() -> int { return 0 }")
	parseErr(t, "import x\nfn main() -> int { return 0 }") // not a string
}

func TestParseQualifiedTypeAndLiteral(t *testing.T) {
	prog := parseOK(t, wrap(`let v: json.Value = json.Value { a: 1 }`))
	body := mainBody(t, prog)
	let, ok := body[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 0 is not a let: %T", body[0])
	}
	if let.Type != ast.TypeName("json.Value") {
		t.Errorf("qualified type = %q, want json.Value", let.Type)
	}
	lit, ok := let.Value.(*ast.StructLit)
	if !ok {
		t.Fatalf("let value is not a struct literal: %T", let.Value)
	}
	if lit.Namespace != "json" || lit.Name != "Value" {
		t.Errorf("qualified struct lit = ns:%q name:%q, want json/Value", lit.Namespace, lit.Name)
	}
	if len(lit.Fields) != 1 || lit.Fields[0].Name != "a" {
		t.Errorf("qualified struct lit fields = %+v", lit.Fields)
	}
}

func TestParseChainedQualifiedTypeErrors(t *testing.T) {
	// The qualified-type grammar is exactly Ident "." Ident; a/b/c-style chaining
	// is rejected so the checker's split-once is sound.
	parseErr(t, wrap(`let v: a.b.c = 0`))
}

func TestParseQualifiedCallStillParses(t *testing.T) {
	// curl.get(url) parses as a FieldAccess-callee CallExpr (the checker resolves
	// whether curl is a namespace). Regression: the postfix path must not turn this
	// into a struct literal (no '{' follows).
	prog := parseOK(t, wrap(`let s: string = curl.get(url)`))
	body := mainBody(t, prog)
	let := body[0].(*ast.LetStmt)
	call, ok := let.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected a CallExpr, got %T", let.Value)
	}
	fa, ok := call.Callee.(*ast.FieldAccess)
	if !ok {
		t.Fatalf("expected a FieldAccess callee, got %T", call.Callee)
	}
	if id, ok := fa.X.(*ast.Ident); !ok || id.Name != "curl" || fa.Field != "get" {
		t.Errorf("callee = %+v", fa)
	}
	if call.CalleeName != "" {
		t.Errorf("CalleeName should be empty for a qualified call, got %q", call.CalleeName)
	}
}

func TestParseExportConstSetsExported(t *testing.T) {
	prog, err := Parse(`export const MAX: int = 3`, "m.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prog.Consts) != 1 {
		t.Fatalf("got %d consts, want 1", len(prog.Consts))
	}
	cd := prog.Consts[0]
	if !cd.Exported {
		t.Errorf("Exported = false, want true")
	}
	if cd.Name != "MAX" || cd.Type != "int" {
		t.Errorf("got name=%q type=%q", cd.Name, cd.Type)
	}
	if cd.ExportPos.Line == 0 {
		t.Errorf("ExportPos not set")
	}
}

func TestParsePlainConstNotExported(t *testing.T) {
	prog, err := Parse(`const X: int = 1`, "m.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if prog.Consts[0].Exported {
		t.Errorf("plain const marked exported")
	}
}

func TestParseExportBlankConstErrors(t *testing.T) {
	_, err := Parse(`export const _: int = 1`, "m.wisp")
	if err == nil {
		t.Fatalf("expected an error for export const _")
	}
	if !strings.Contains(err.Error(), "export const") {
		t.Errorf("error = %q, want it to mention export const", err.Error())
	}
}
