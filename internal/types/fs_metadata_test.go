package types

import "testing"

// Filesystem metadata builtins (fs-metadata task 1): type-checker coverage for
// fs.is_file/fs.is_symlink (string)->bool and fs.file_size (string)->int.
// is_file/is_symlink/file_size/chmod/symlink/read_link/temp_file/temp_dir/glob
// are all removable builtins now namespaced under fs, so every test below
// checks through the linked module set with the fs namespace bound.
// symlink_force and pid_alive are also removable (fs.symlink_force /
// process.pid_alive).

// --- well-typed results ---

func TestFSMeta_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let a: bool = fs.is_file("x")
let b: bool = fs.is_symlink("x")
let n: int = fs.file_size("x")`), "fs")
}

// --- argument typing: is_file ---

func TestFSMeta_IsFile_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.is_file(1)`), "fs.is_file", "fs")
}

func TestFSMeta_IsFile_RequiresArg(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.is_file()`), "fs.is_file", "fs")
}

// --- argument typing: is_symlink ---

func TestFSMeta_IsSymlink_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.is_symlink(1)`), "fs.is_symlink", "fs")
}

func TestFSMeta_IsSymlink_RequiresArg(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: bool = fs.is_symlink()`), "fs.is_symlink", "fs")
}

// --- argument typing: file_size ---

func TestFSMeta_FileSize_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let n: int = fs.file_size(1)`), "fs.file_size", "fs")
}

func TestFSMeta_FileSize_RequiresArg(t *testing.T) {
	expectErrNS(t, wrapMain(`let n: int = fs.file_size()`), "fs.file_size", "fs")
}

// --- names freed for user binding ---

// is_file/is_symlink/file_size are removable builtins: their flat names were
// freed by the modules-only migration (isReservedName excludes the removable
// set), so they are now ordinary identifiers a user may bind -- unlike the
// pre-removal original, which reserved them as bare builtin names.
var fsMetaTask1Names = []string{"is_file", "is_symlink", "file_size"}

func TestFSMeta_Task1_NamesFreed_Fn(t *testing.T) {
	for _, name := range fsMetaTask1Names {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestFSMeta_Task1_NamesFreed_Let(t *testing.T) {
	for _, name := range fsMetaTask1Names {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestFSMeta_Task1_NamesFreed_Param(t *testing.T) {
	for _, name := range fsMetaTask1Names {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

// --- Task 2: fs.chmod/fs.symlink (string,string)->Void ---

// well-typed: chmod and symlink used as statements (Void)
func TestFSMeta_Task2_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let f: string = "p"
fs.chmod(f, "644")
fs.symlink(f, "link")`), "fs")
}

// --- argument typing: chmod ---

func TestFSMeta_Chmod_RequiresTwoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.chmod("p")`), "fs.chmod", "fs")
}

func TestFSMeta_Chmod_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.chmod(1, "644")`), "fs.chmod", "fs")
}

func TestFSMeta_Chmod_ModeMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.chmod("p", 644)`), "fs.chmod", "fs")
}

// --- argument typing: symlink ---

func TestFSMeta_Symlink_RequiresTwoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.symlink("a")`), "fs.symlink", "fs")
}

func TestFSMeta_Symlink_TargetMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.symlink(1, "link")`), "fs.symlink", "fs")
}

func TestFSMeta_Symlink_LinkPathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`fs.symlink("a", 2)`), "fs.symlink", "fs")
}

// --- Void not usable as a value ---

func TestFSMeta_Chmod_VoidNotAValue(t *testing.T) {
	expectErrNS(t, wrapMain(`let x: string = fs.chmod("p", "644")`), "void", "fs")
}

func TestFSMeta_Symlink_VoidNotAValue(t *testing.T) {
	expectErrNS(t, wrapMain(`let y: string = fs.symlink("a", "b")`), "void", "fs")
}

// --- names freed for user binding ---

var fsMetaTask2Names = []string{"chmod", "symlink"}

func TestFSMeta_Task2_NamesFreed_Fn(t *testing.T) {
	for _, name := range fsMetaTask2Names {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestFSMeta_Task2_NamesFreed_Let(t *testing.T) {
	for _, name := range fsMetaTask2Names {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestFSMeta_Task2_NamesFreed_Param(t *testing.T) {
	for _, name := range fsMetaTask2Names {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

// --- Task 3: fs.read_link/fs.temp_file/fs.temp_dir ---

// well-typed results
func TestFSMeta_Task3_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let r: Optional[string] = fs.read_link("s")
let f: string = fs.temp_file()
let d: string = fs.temp_dir()`), "fs")
}

// --- argument typing: read_link ---

func TestFSMeta_ReadLink_PathMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: Optional[string] = fs.read_link(1)`), "fs.read_link", "fs")
}

func TestFSMeta_ReadLink_RequiresArg(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: Optional[string] = fs.read_link()`), "fs.read_link", "fs")
}

// --- argument typing: temp_file ---

func TestFSMeta_TempFile_TakesNoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`let f: string = fs.temp_file(1)`), "fs.temp_file", "fs")
}

// --- argument typing: temp_dir ---

func TestFSMeta_TempDir_TakesNoArgs(t *testing.T) {
	expectErrNS(t, wrapMain(`let d: string = fs.temp_dir(1)`), "fs.temp_dir", "fs")
}

// --- names freed for user binding ---

var fsMetaTask3Names = []string{"read_link", "temp_file", "temp_dir"}

func TestFSMeta_Task3_NamesFreed_Fn(t *testing.T) {
	for _, name := range fsMetaTask3Names {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestFSMeta_Task3_NamesFreed_Let(t *testing.T) {
	for _, name := range fsMetaTask3Names {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestFSMeta_Task3_NamesFreed_Param(t *testing.T) {
	for _, name := range fsMetaTask3Names {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

// --- Task 4: fs.glob (string)->string[] ---

// well-typed: glob returns string[]; a length/index use confirms the element type.
func TestFSMeta_Task4_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let xs: string[] = fs.glob("*.txt")
let n: int = length(xs)
let head: string = xs[0]`), "fs")
}

// --- argument typing: glob ---

func TestFSMeta_Glob_PatternMustBeString(t *testing.T) {
	expectErrNS(t, wrapMain(`let xs: string[] = fs.glob(1)`), "fs.glob", "fs")
}

func TestFSMeta_Glob_RequiresArg(t *testing.T) {
	expectErrNS(t, wrapMain(`let xs: string[] = fs.glob()`), "fs.glob", "fs")
}

// --- names freed for user binding ---

var fsMetaTask4Names = []string{"glob"}

func TestFSMeta_Task4_NamesFreed_Fn(t *testing.T) {
	for _, name := range fsMetaTask4Names {
		src := "fn " + name + "() -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

func TestFSMeta_Task4_NamesFreed_Let(t *testing.T) {
	for _, name := range fsMetaTask4Names {
		expectOKNS(t, wrapMain("let "+name+": int = 0"), "fs")
	}
}

func TestFSMeta_Task4_NamesFreed_Param(t *testing.T) {
	for _, name := range fsMetaTask4Names {
		src := "fn f(" + name + ": int) -> int { return 0 }\nfn main() -> int { return 0 }"
		expectOKNS(t, src, "fs")
	}
}

// --- fs.symlink_force ---

func TestSymlinkForce_Checks(t *testing.T) {
	expectOKNS(t, `fn main() -> int { fs.symlink_force("t","l"); return 0 }`, "fs")
	expectOKNS(t, `fn main() -> int { fs.symlink_force("t","l"); print("after"); return 0 }`, "fs") // not a terminator

	expectErrNS(t, `fn main() -> int { fs.symlink_force("a"); return 0 }`, "expects", "fs")
	expectErrNS(t, `fn main() -> int { fs.symlink_force("a","b","c"); return 0 }`, "expects", "fs")
	expectErrNS(t, `fn main() -> int { fs.symlink_force(1,"b"); return 0 }`, "has type", "fs")
	expectErrNS(t, `fn main() -> int { fs.symlink_force("a",2); return 0 }`, "has type", "fs")
	expectErrNS(t, `fn main() -> int { let x: string = fs.symlink_force("a","b"); return 0 }`, "void", "fs")

	// symlink_force's flat name is freed for user binding (removable builtin).
	expectOKNS(t, `fn symlink_force() -> int { return 0 } fn main() -> int { return 0 }`, "fs")
	expectOKNS(t, `fn main() -> int { let symlink_force: int = 1; return 0 }`, "fs")
}

// --- process.pid_alive ---

func TestPidAlive_Checks(t *testing.T) {
	expectOKNS(t, `fn main() -> int { let b: bool = process.pid_alive(123); return 0 }`, "process")
	expectOKNS(t, "fn main() -> int {\nif (process.pid_alive(1)) { print(\"x\") }\nreturn 0\n}", "process")
	expectOKNS(t, `fn main() -> int { let b: bool = process.pid_alive(1); print("after"); return 0 }`, "process") // not a terminator

	expectErrNS(t, `fn main() -> int { let b: bool = process.pid_alive(); return 0 }`, "expects", "process")
	expectErrNS(t, `fn main() -> int { let b: bool = process.pid_alive(1,2); return 0 }`, "expects", "process")
	expectErrNS(t, `fn main() -> int { let b: bool = process.pid_alive("x"); return 0 }`, "has type", "process")
	expectErrNS(t, `fn main() -> int { let b: bool = process.pid_alive(true); return 0 }`, "has type", "process")

	// pid_alive's flat name is freed for user binding (removable builtin).
	expectOKNS(t, `fn pid_alive() -> int { return 0 } fn main() -> int { return 0 }`, "process")
	expectOKNS(t, `fn main() -> int { let pid_alive: int = 1; return 0 }`, "process")
}

func TestPidAlive_ReferenceAllowed(t *testing.T) {
	// process.pid_alive is a total monomorphic builtin: (int) -> bool. Under
	// universal funcrefs it is referenceable and records that funcref type.
	expectOKNS(t, `fn main()->int{ let f: fn(int)->bool = process.pid_alive; return 0 }`, "process")
}
