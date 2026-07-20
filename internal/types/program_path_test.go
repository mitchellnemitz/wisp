package types

import "testing"

// fs.program_path()/fs.dir_name()/fs.base_name() (language-gaps Task 4):
// type-checker coverage. program_path() takes no args and returns string;
// dir_name/base_name take one string and return string. All three are
// removable builtins now namespaced under fs, so every test below checks
// through the linked module set with the fs namespace bound. Negative cases
// assert the located compile error; name-freed cases mirror the fs set.

func TestProgramPath_Builtins_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let p: string = fs.program_path()
let d: string = fs.dir_name(p)
let b: string = fs.base_name(p)
let nested: string = fs.dir_name(fs.program_path())`), "fs")
}

func TestProgramPath_TakesNoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`let p: string = fs.program_path("x")`), "fs.program_path", "fs")
}

func TestProgramPath_ResultIsString(t *testing.T) {
	expectErrNS(t, wrapMain(`let n: int = fs.program_path()`), "int", "fs")
}

func TestDirName_ArgMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let d: string = fs.dir_name(1)`), "fs.dir_name", "fs")
}

func TestDirName_ArgCount(t *testing.T) {
	expectErrNS(t, wrapMain(`let d: string = fs.dir_name("a", "b")`), "fs.dir_name", "fs")
}

func TestBaseName_ArgMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let b: string = fs.base_name(1)`), "fs.base_name", "fs")
}

func TestBaseName_ArgCount(t *testing.T) {
	expectErrNS(t, wrapMain(`let b: string = fs.base_name()`), "fs.base_name", "fs")
}

// program_path/dir_name/base_name are removable builtins: their flat names
// were freed by the modules-only migration (isReservedName excludes the
// removable set), so they are now ordinary identifiers a user may bind --
// unlike the pre-removal original, which reserved them as bare builtin names.
var programPathNames = []string{"program_path", "dir_name", "base_name"}

func TestProgramPath_NamesFreed_Fn(t *testing.T) {
	for _, name := range programPathNames {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestProgramPath_NamesFreed_Let(t *testing.T) {
	for _, name := range programPathNames {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestProgramPath_NamesFreed_Param(t *testing.T) {
	for _, name := range programPathNames {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}
