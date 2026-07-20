package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

func checkFSProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"fs": 1})
	fm := coreMod(1, "fs")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, fm}})
}

func TestCoreFSMembersResolve(t *testing.T) {
	optStr := optionalType(String)
	arrStr := arrayType(String)
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let b: bool = fs.file_exists("p"); return 0 }`, "file_exists", Bool},
		{`fn main() -> int { let b: bool = fs.is_dir("p"); return 0 }`, "is_dir", Bool},
		{`fn main() -> int { let b: bool = fs.is_file("p"); return 0 }`, "is_file", Bool},
		{`fn main() -> int { let b: bool = fs.is_symlink("p"); return 0 }`, "is_symlink", Bool},
		{`fn main() -> int { fs.chmod("p", "755"); return 0 }`, "chmod", Void},
		{`fn main() -> int { fs.symlink("t", "l"); return 0 }`, "symlink", Void},
		{`fn main() -> int { fs.symlink_force("t", "l"); return 0 }`, "symlink_force", Void},
		{`fn main() -> int { let i: int = fs.file_size("p"); return 0 }`, "file_size", Int},
		{`fn main() -> int { let s: string = fs.cwd(); return 0 }`, "cwd", String},
		{`fn main() -> int { let s: string = fs.program_path(); return 0 }`, "program_path", String},
		{`fn main() -> int { let s: string = fs.dir_name("a/b"); return 0 }`, "dir_name", String},
		{`fn main() -> int { let s: string = fs.base_name("a/b"); return 0 }`, "base_name", String},
		{`fn main() -> int { fs.make_dir("p"); return 0 }`, "make_dir", Void},
		{`fn main() -> int { fs.remove_file("p"); return 0 }`, "remove_file", Void},
		{`fn main() -> int { fs.remove_dir("p"); return 0 }`, "remove_dir", Void},
		{`fn main() -> int { fs.rename("a", "b"); return 0 }`, "rename", Void},
		{`fn main() -> int { let o: Optional[string] = fs.which("ls"); return 0 }`, "which", optStr},
		{`fn main() -> int { let o: Optional[string] = fs.read_link("p"); return 0 }`, "read_link", optStr},
		{`fn main() -> int { let s: string = fs.temp_file(); return 0 }`, "temp_file", String},
		{`fn main() -> int { let s: string = fs.temp_dir(); return 0 }`, "temp_dir", String},
		{`fn main() -> int { let a: string[] = fs.list_dir("p"); return 0 }`, "list_dir", arrStr},
		{`fn main() -> int { let a: string[] = fs.glob("*.x"); return 0 }`, "glob", arrStr},
		{`fn main() -> int { let s: string = fs.read_file("p"); return 0 }`, "read_file", String},
		{`fn main() -> int { fs.write_file("p", "c"); return 0 }`, "write_file", Void},
		{`fn main() -> int { fs.append_file("p", "c"); return 0 }`, "append_file", Void},
		{`fn main() -> int { fs.change_dir("p"); return 0 }`, "change_dir", Void},
	} {
		info := checkFSProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

func TestCoreFSAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let s: string = f.read_file("p"); return 0 }`, map[string]int{"f": 1})
	fm := coreMod(1, "fs")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, fm}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased f.read_file should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "read_file"); ci == nil || ci.Result != String {
		t.Errorf("f.read_file result = %v, want string", ci)
	}
}

func TestCoreFSArgTypeErrorNamesMember(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let s: string = fs.read_file(1); return 0 }`)
	if !hasErr(info, "argument 1 of fs.read_file has type int, want string") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

func TestCoreFSTypeArgsRejected(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let s: string = fs.read_file[int]("p"); return 0 }`)
	if !hasErr(info, "fs.read_file does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreFSUnknownMemberSuggestion(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let s: string = fs.read_fil("p"); return 0 }`)
	if !hasErr(info, `did you mean "read_file"?`) {
		t.Fatalf("want read_file suggestion, got %v", errMsgs(info))
	}
}

// TestCoreFSFuncrefValueAllowed pins Part 3: a core-module member whose
// underlying builtin is generatable IS referenceable as a funcref VALUE. It
// records a MemberFuncRef minting the same __wisp_builtin_<name> wrapper the
// bare-ident path uses. Members whose builtin is not generatable (e.g. fs.glob,
// fs.which) still error; see TestCoreFSFuncrefValueRejectedForNonGeneratable.
func TestCoreFSFuncrefValueAllowed(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let f: fn(string)->bool = fs.is_file; let _: bool = f("/x"); return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("fs.is_file funcref should type-check; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for fs.is_file")
	}
}

// TestCoreFSFuncrefValueRejectedForNonGeneratable pins that a member whose
// underlying builtin is NOT in the generatable allowlist (composite result)
// still gets the wrong-kind diagnostic in value position.
func TestCoreFSFuncrefValueRejectedForNonGeneratable(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let f: fn(string)->string = fs.glob; return 0 }`)
	if !hasErr(info, `"glob" of module "fs" cannot be referenced as a function value (it has no single funcref-shaped scalar lowering); wrap it in a fn`) {
		t.Fatalf("want func-as-value error, got %v", errMsgs(info))
	}
}

// TestCoreFSFuncrefValueProgramPath pins that fs.program_path (a nullary
// total-helper delegate, now generatable) is referenceable as a funcref value.
func TestCoreFSFuncrefValueProgramPath(t *testing.T) {
	info := checkFSProg(t, `fn main() -> int { let f: fn()->string = fs.program_path; return 0 }`)
	if len(info.Errors) != 0 {
		t.Fatalf("fs.program_path funcref should type-check; errors: %v", errMsgs(info))
	}
	if len(info.MemberFuncRefs) == 0 {
		t.Fatalf("expected a MemberFuncRef recorded for fs.program_path")
	}
}
