package types

import "testing"

// process.run_env/run_env_status/run_env_full are removable builtins (bare
// run_env* no longer resolves in the single-module check), so every test
// below checks through the linked module set with the process namespace
// bound, via expectOKNS/expectErrNS.

// --- process.run_env(argv: string[], env: {string:string}) -> string ---

func TestRunEnv_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let out: string = process.run_env(["true"], {"FOO": "bar"})`), "process")
}

func TestRunEnv_ResultIsString(t *testing.T) {
	expectOKNS(t, wrapMain(`let xs: string[] = ["echo", "hi"]
let e: {string: string} = {"K": "v"}
let out: string = process.run_env(xs, e)`), "process")
}

func TestRunEnv_EmptyEnvDict_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let e: {string: string} = {}
let out: string = process.run_env(["true"], e)`), "process")
}

func TestRunEnv_WrongArity_One(t *testing.T) {
	expectErrNS(t, wrapMain(`let out: string = process.run_env(["true"])`), "run_env", "process")
}

func TestRunEnv_WrongArity_Three(t *testing.T) {
	expectErrNS(t, wrapMain(`let out: string = process.run_env(["a"], {"K": "v"}, ["b"])`), "run_env", "process")
}

func TestRunEnv_Arg0MustBeStringArray(t *testing.T) {
	expectErrNS(t, wrapMain(`let out: string = process.run_env("echo", {"K": "v"})`), "run_env", "process")
}

func TestRunEnv_Arg0MustBeStringElem(t *testing.T) {
	expectErrNS(t, wrapMain(`let out: string = process.run_env([1, 2], {"K": "v"})`), "run_env", "process")
}

func TestRunEnv_Arg1MustBeDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let out: string = process.run_env(["true"], ["a"])`), "run_env", "process")
}

func TestRunEnv_Arg1MustBeStringStringDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let e: {int: int} = {1: 2}
let out: string = process.run_env(["true"], e)`), "run_env", "process")
}

// run_env is a removable builtin: its flat name was freed by the
// modules-only migration (isReservedName excludes the removable set), so it
// is now an ordinary identifier a user may bind -- unlike the pre-removal
// original, which reserved it as a bare builtin name.
func TestRunEnv_NameFreedForUserBinding(t *testing.T) {
	expectOKNS(t, "fn run_env() -> int { return 0 }\nfn main() -> int { return 0 }", "process")
	expectOKNS(t, wrapMain(`let run_env: int = 0`), "process")
	expectOKNS(t, "fn f(run_env: int) -> int { return 0 }\nfn main() -> int { return 0 }", "process")
}

// --- process.run_env_status(argv: string[], env: {string:string}) -> int ---

func TestRunEnvStatus_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let rc: int = process.run_env_status(["true"], {"FOO": "bar"})`), "process")
}

func TestRunEnvStatus_ResultIsInt(t *testing.T) {
	expectOKNS(t, wrapMain(`let xs: string[] = ["sh", "-c", "exit 3"]
let e: {string: string} = {"K": "v"}
let rc: int = process.run_env_status(xs, e)`), "process")
}

func TestRunEnvStatus_EmptyEnvDict_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let e: {string: string} = {}
let rc: int = process.run_env_status(["true"], e)`), "process")
}

func TestRunEnvStatus_WrongArity_One(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_env_status(["true"])`), "run_env_status", "process")
}

func TestRunEnvStatus_WrongArity_Three(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_env_status(["a"], {"K": "v"}, ["b"])`), "run_env_status", "process")
}

func TestRunEnvStatus_Arg0MustBeStringArray(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_env_status("echo", {"K": "v"})`), "run_env_status", "process")
}

func TestRunEnvStatus_Arg0MustBeStringElem(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_env_status([1, 2], {"K": "v"})`), "run_env_status", "process")
}

func TestRunEnvStatus_Arg1MustBeDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let rc: int = process.run_env_status(["true"], ["a"])`), "run_env_status", "process")
}

func TestRunEnvStatus_Arg1MustBeStringStringDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let e: {int: int} = {1: 2}
let rc: int = process.run_env_status(["true"], e)`), "run_env_status", "process")
}

func TestRunEnvStatus_NameFreedForUserBinding(t *testing.T) {
	expectOKNS(t, "fn run_env_status() -> int { return 0 }\nfn main() -> int { return 0 }", "process")
	expectOKNS(t, wrapMain(`let run_env_status: int = 0`), "process")
	expectOKNS(t, "fn f(run_env_status: int) -> int { return 0 }\nfn main() -> int { return 0 }", "process")
}

// --- process.run_env_full(argv: string[], env: {string:string}) -> RunResult ---

func TestRunEnvFull_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let r: RunResult = process.run_env_full(["true"], {"FOO": "bar"})`), "process")
}

func TestRunEnvFull_ResultIsRunResult(t *testing.T) {
	expectOKNS(t, wrapMain(`let xs: string[] = ["sh", "-c", "echo hi"]
let e: {string: string} = {"K": "v"}
let r: RunResult = process.run_env_full(xs, e)`), "process")
}

func TestRunEnvFull_EmptyEnvDict_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let e: {string: string} = {}
let r: RunResult = process.run_env_full(["true"], e)`), "process")
}

func TestRunEnvFull_WrongArity_One(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: RunResult = process.run_env_full(["true"])`), "run_env_full", "process")
}

func TestRunEnvFull_WrongArity_Three(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: RunResult = process.run_env_full(["a"], {"K": "v"}, ["b"])`), "run_env_full", "process")
}

func TestRunEnvFull_Arg0MustBeStringArray(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: RunResult = process.run_env_full("echo", {"K": "v"})`), "run_env_full", "process")
}

func TestRunEnvFull_Arg0MustBeStringElem(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: RunResult = process.run_env_full([1, 2], {"K": "v"})`), "run_env_full", "process")
}

func TestRunEnvFull_Arg1MustBeDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let r: RunResult = process.run_env_full(["true"], ["a"])`), "run_env_full", "process")
}

func TestRunEnvFull_Arg1MustBeStringStringDict(t *testing.T) {
	expectErrNS(t, wrapMain(`let e: {int: int} = {1: 2}
let r: RunResult = process.run_env_full(["true"], e)`), "run_env_full", "process")
}

func TestRunEnvFull_NameFreedForUserBinding(t *testing.T) {
	expectOKNS(t, "fn run_env_full() -> int { return 0 }\nfn main() -> int { return 0 }", "process")
	expectOKNS(t, wrapMain(`let run_env_full: int = 0`), "process")
	expectOKNS(t, "fn f(run_env_full: int) -> int { return 0 }\nfn main() -> int { return 0 }", "process")
}
