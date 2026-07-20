package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
)

func checkProcessProg(t *testing.T, rootSrc string) *Info {
	t.Helper()
	root := mod(t, 0, rootSrc, map[string]int{"process": 1})
	pm := coreMod(1, "process")
	return CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
}

func TestCoreProcessMembersResolve(t *testing.T) {
	for _, c := range []struct {
		src     string
		builtin string
		want    Type
	}{
		{`fn main() -> int { let s: string = process.run(["echo", "hi"]); return 0 }`, "run", String},
		{`fn main() -> int { let i: int = process.run_status(["true"]); return 0 }`, "run_status", Int},
		{`fn main() -> int { let r: RunResult = process.run_full(["echo"]); return 0 }`, "run_full", RunResult},
		{`fn main() -> int { let s: string = process.run_env(["x"], {"K": "v"}); return 0 }`, "run_env", String},
		{`fn main() -> int { let i: int = process.run_env_status(["x"], {"K": "v"}); return 0 }`, "run_env_status", Int},
		{`fn main() -> int { let r: RunResult = process.run_env_full(["x"], {"K": "v"}); return 0 }`, "run_env_full", RunResult},
		{`fn main() -> int { let s: string = process.run_input(["cat"], "in"); return 0 }`, "run_input", String},
		{`fn main() -> int { let r: RunResult = process.run_input_full(["cat"], "in"); return 0 }`, "run_input_full", RunResult},
		{`fn main() -> int { process.exec_command(["x"]); return 0 }`, "exec_command", Void},
		{`fn main() -> int { let r: RunResult = process.pipe([["echo", "hi"], ["cat"]]); return 0 }`, "pipe", RunResult},
		{`fn main() -> int { let p: Process = process.spawn(["sleep", "1"]); return 0 }`, "spawn", Process},
		{`fn main() -> int { let b: bool = process.pid_alive(123); return 0 }`, "pid_alive", Bool},
		{`fn main() -> int { process.make_fifo("p"); return 0 }`, "make_fifo", Void},
	} {
		info := checkProcessProg(t, c.src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

// Members taking a Process handle need one constructed first.
func TestCoreProcessHandleMembers(t *testing.T) {
	for _, c := range []struct {
		body    string
		builtin string
		want    Type
	}{
		{`let r: RunResult = process.wait(p)`, "wait", RunResult},
		{`let b: bool = process.is_done(p)`, "is_done", Bool},
		{`process.signal(p, "TERM")`, "signal", Void},
		{`let ps: Process[] = [p]; let w: Process = process.wait_any(ps, 1)`, "wait_any", Process},
	} {
		src := `fn main() -> int { let p: Process = process.spawn(["sleep", "1"]); ` + c.body + `; return 0 }`
		info := checkProcessProg(t, src)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: unexpected errors: %v", c.builtin, errMsgs(info))
		}
		if ci := callWithBuiltin(info, c.builtin); ci == nil || ci.Result != c.want {
			t.Errorf("%s: got %v, want result %q", c.builtin, ci, c.want)
		}
	}
}

func TestCoreProcessWaitAnyDomainCheck(t *testing.T) {
	src := `fn main() -> int { let p: Process = process.spawn(["x"]); let ps: Process[] = [p]; let w: Process = process.wait_any(ps, -1); return 0 }`
	info := checkProcessProg(t, src)
	if !hasErr(info, "process.wait_any: poll_secs must be >= 0") {
		t.Fatalf("want wait_any domain error via delegation, got %v", errMsgs(info))
	}
}

func TestCoreProcessAliasImport(t *testing.T) {
	root := mod(t, 0, `fn main() -> int { let s: string = p.run(["echo"]); return 0 }`, map[string]int{"p": 1})
	pm := coreMod(1, "process")
	info := CheckLinked(&module.Linked{Modules: []*module.Module{root, pm}})
	if len(info.Errors) != 0 {
		t.Fatalf("aliased p.run should resolve; errors: %v", errMsgs(info))
	}
	if ci := callWithBuiltin(info, "run"); ci == nil || ci.Result != String {
		t.Errorf("p.run result = %v, want string", ci)
	}
}

func TestCoreProcessTypeArgsRejected(t *testing.T) {
	info := checkProcessProg(t, `fn main() -> int { let s: string = process.run[int](["echo"]); return 0 }`)
	if !hasErr(info, "process.run does not take type arguments") {
		t.Fatalf("want type-arg rejection, got %v", errMsgs(info))
	}
}

func TestCoreProcessPidAliveArgTypeNamesMember(t *testing.T) {
	// pid_alive is the one coreSig member -> arg-type error names process.pid_alive.
	info := checkProcessProg(t, `fn main() -> int { let b: bool = process.pid_alive("x"); return 0 }`)
	if !hasErr(info, "argument 1 of process.pid_alive has type string, want int") {
		t.Fatalf("want member-named arg-type error, got %v", errMsgs(info))
	}
}

func TestCoreProcessUnknownMemberSuggestion(t *testing.T) {
	info := checkProcessProg(t, `fn main() -> int { let s: string = process.ru(["echo"]); return 0 }`)
	if !hasErr(info, `did you mean "run"?`) {
		t.Fatalf("want run suggestion, got %v", errMsgs(info))
	}
}
