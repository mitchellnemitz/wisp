package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// aliasOf parses a single top-level `type <name> = <rhs>` declaration and
// returns it.
func aliasOf(t *testing.T, src string) *ast.TypeAliasDecl {
	t.Helper()
	prog := parseOK(t, src)
	if len(prog.Aliases) != 1 {
		t.Fatalf("parse(%q): expected 1 alias, got %d", src, len(prog.Aliases))
	}
	return prog.Aliases[0]
}

func TestParseTypeAliasRHS(t *testing.T) {
	cases := []struct {
		rhs  string
		want ast.TypeName
	}{
		{"int", ast.TypeInt},
		{"string[]", ast.ArrayType(ast.TypeString)},
		{"int[][]", ast.ArrayType(ast.ArrayType(ast.TypeInt))},
		{"{string: string}", ast.DictType(ast.TypeString, ast.TypeString)},
		{"(int, int)", ast.TupleType([]ast.TypeName{ast.TypeInt, ast.TypeInt})},
		{"fn(int, int) -> int", ast.FuncType([]ast.TypeName{ast.TypeInt, ast.TypeInt}, ast.TypeInt)},
		{"fn() -> void", ast.FuncType(nil, ast.TypeVoid)},
		{"Optional[int]", ast.OptionalType(ast.TypeInt)},
		{"Result[int]", ast.ResultType(ast.TypeInt)},
		{"Box[int]", ast.TypeName("Box[int]")},
		{"geo.Point", ast.TypeName("geo.Point")},
		{"(int)", ast.TypeInt}, // grouping unwraps
		{"error", ast.TypeName("error")},
	}
	for _, c := range cases {
		a := aliasOf(t, "type Alias = "+c.rhs)
		if a.Name != "Alias" {
			t.Errorf("type Alias = %q: name = %q, want Alias", c.rhs, a.Name)
		}
		if a.Type != c.want {
			t.Errorf("type Alias = %q: Type = %q, want %q", c.rhs, a.Type, c.want)
		}
	}
}

// TestParseTypeAliasAsAtom: an alias name used in type position flows through
// the postfix array loop, so `MyAlias[]` encodes as array-of-alias.
func TestParseTypeAliasAsAtom(t *testing.T) {
	a := aliasOf(t, "type AA = Miles[]")
	if a.Type != ast.ArrayType(ast.TypeName("Miles")) {
		t.Errorf("Type = %q, want %q", a.Type, ast.ArrayType(ast.TypeName("Miles")))
	}
}

func TestParseTypeAliasErrors(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"type X int", "expected ="},
		{"type = int", "expected"},
		{"type X = void", "void is only valid as a return type"},
		{"type Pair[T] = (T, T)", "generic type aliases are not supported"},
		{"export type X = int", "type aliases are module-local and cannot be exported"},
		{wrap("type X = int"), "expected"}, // `type` inside a function body
	}
	for _, c := range cases {
		err := parseErr(t, c.src)
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("parse(%q) error = %q, want substring %q", c.src, err.Error(), c.want)
		}
	}
}

// TestParseTypeAliasBlankParses: `type _ = int` PARSES (the checker rejects the
// blank name, not the parser).
func TestParseTypeAliasBlankParses(t *testing.T) {
	a := aliasOf(t, "type _ = int")
	if a.Name != "_" {
		t.Errorf("name = %q, want _", a.Name)
	}
}
