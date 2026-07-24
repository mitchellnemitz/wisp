package types

import "testing"

// Filesystem + process stdlib (fs milestone): type-checker coverage for the
// non-array fixed-signature builtins added in Task 1 (file_exists, is_dir, cwd)
// plus the void mutating ops (make_dir, remove_file, remove_dir, rename). All
// are removable builtins now spelled fs.file_exists / fs.is_dir / fs.cwd /
// fs.make_dir / fs.remove_file / fs.remove_dir / fs.rename, so every test
// below checks through the linked module set with the fs (and, for
// run_status, process) namespace bound. Positive cases assert the result type
// flows; negative cases assert the located compile error; name-freed cases
// mirror the M7 pattern (a removable builtin's bare name is no longer
// reserved). env_or itself is gone entirely (Task 6): its arg-type/arg-count
// checking is covered by env.get's own tests in core_env_test.go, since
// unwrap_or(env.get(n), fb) is the replacement.

// --- well-typed results ---

func TestFS_Task1Builtins_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let a: bool = fs.file_exists("x")
let b: bool = fs.is_dir("x")
let c: string = fs.cwd()
let d: string = unwrap_or(env.get("HOME"), "fb")
fs.make_dir("x")
fs.remove_file("x")
fs.remove_dir("x")
fs.rename("a", "b")`), "fs", "env")
}

// --- argument typing ---

func TestFS_FileExists_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.file_exists(1)`), "fs.file_exists", "fs")
}

func TestFS_IsDir_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.is_dir(1)`), "fs.is_dir", "fs")
}

func TestFS_Cwd_TakesNoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`let c: string = fs.cwd("x")`), "fs.cwd", "fs")
}

func TestFS_MakeDir_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.make_dir(1)`), "fs.make_dir", "fs")
}

func TestFS_RemoveFile_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.remove_file(1)`), "fs.remove_file", "fs")
}

func TestFS_RemoveDir_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.remove_dir(1)`), "fs.remove_dir", "fs")
}

func TestFS_Rename_ArgsMustBeStrings(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.rename(1, "b")`), "fs.rename", "fs")
	expectErrNS(t, wrapMain(`fs.rename("a", 1)`), "fs.rename", "fs")
}

// --- result types ---

func TestFS_MutatingOps_AreVoid(t *testing.T) {
	expectErrNS(t, wrapMain(`let x: string = fs.make_dir("d")`), "void", "fs")
	expectErrNS(t, wrapMain(`let x: string = fs.remove_file("f")`), "void", "fs")
	expectErrNS(t, wrapMain(`let x: string = fs.remove_dir("d")`), "void", "fs")
	expectErrNS(t, wrapMain(`let x: string = fs.rename("a", "b")`), "void", "fs")
}

// --- names freed for user binding (all seven) ---

// file_exists/is_dir/cwd/make_dir/remove_file/remove_dir/rename are
// removable builtins: their flat names were freed by the modules-only
// migration (isReservedName excludes the removable set), so they are now
// ordinary identifiers a user may bind -- unlike the pre-removal original,
// which reserved them as bare builtin names. env_or is not in this list: it
// was fully removed (Task 6), not merely moved to a module, so it was never a
// "removable builtin" in this test's sense to begin with.
var fsTask1Names = []string{"file_exists", "is_dir", "cwd", "make_dir", "remove_file", "remove_dir", "rename"}

func TestFS_Task1_NamesFreed_Fn(t *testing.T) {
	for _, name := range fsTask1Names {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestFS_Task1_NamesFreed_Let(t *testing.T) {
	for _, name := range fsTask1Names {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestFS_Task1_NamesFreed_Param(t *testing.T) {
	for _, name := range fsTask1Names {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

// --- fs.which(name: string) -> Optional[string] (Task 2) ---

func TestFS_Which_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let p: Optional[string] = fs.which("sh")`), "fs")
}

func TestFS_Which_NameMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let p: Optional[string] = fs.which(1)`), "fs.which", "fs")
}

func TestFS_Which_ArgCount(t *testing.T) {
	expectErrNS(t, wrapMain(`let p: Optional[string] = fs.which("a", "b")`), "fs.which", "fs")
}

func TestFS_Which_NameFreed(t *testing.T) {
	expectOKNS(t, "fn which() -> int { return 0 }\nfn main() -> int { return 0 }", "fs")
	expectOKNS(t, wrapMain(`let which: int = 0`), "fs")
	expectOKNS(t, "fn f(which: int) -> int { return 0 }\nfn main() -> int { return 0 }", "fs")
}

// --- fs.list_dir(path: string) -> string[] (Task 3) ---

func TestFS_ListDir_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let xs: string[] = fs.list_dir("d")`), "fs")
}

func TestFS_ListDir_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let xs: string[] = fs.list_dir(1)`), "fs.list_dir", "fs")
}

func TestFS_ListDir_ArgCount(t *testing.T) {
	expectErrNS(t, wrapMain(`let xs: string[] = fs.list_dir("a", "b")`), "fs.list_dir", "fs")
}

func TestFS_ListDir_NameFreed(t *testing.T) {
	expectOKNS(t, "fn list_dir() -> int { return 0 }\nfn main() -> int { return 0 }", "fs")
	expectOKNS(t, wrapMain(`let list_dir: int = 0`), "fs")
	expectOKNS(t, "fn f(list_dir: int) -> int { return 0 }\nfn main() -> int { return 0 }", "fs")
}

// --- process.run_status(argv: string[]) -> int (Task 4) ---

func TestFS_RunStatus_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let rc: int = process.run_status(["true"])`), "process")
}

func TestFS_RunStatus_ArgMustBeStringArray(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_status([1, 2])`), "run_status", "process")
}

func TestFS_RunStatus_ArgMustBeArray(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_status("echo")`), "run_status", "process")
}

func TestFS_RunStatus_ArgCount(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_status(["a"], ["b"])`), "run_status", "process")
}

func TestFS_RunStatus_NameFreed(t *testing.T) {
	expectOKNS(t, "fn run_status() -> int { return 0 }\nfn main() -> int { return 0 }", "process")
	expectOKNS(t, wrapMain(`let run_status: int = 0`), "process")
	expectOKNS(t, "fn f(run_status: int) -> int { return 0 }\nfn main() -> int { return 0 }", "process")
}
