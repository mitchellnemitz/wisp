package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// M7 (I/O) codegen: compile-and-run behavioral tests for env, has_env,
// read_file, write_file, append_file, run, exit. File fixtures run in a
// per-test temp working directory so writes are isolated. env/has_env/
// read_file/write_file/append_file/run are removable builtins now spelled
// env.get / env.has / fs.read_file / fs.write_file / fs.append_file /
// process.run; every test using one of them compiles through runNSDir/compileNS
// with the namespaces it needs. exit stays flat.

// runWispDir compiles, shellchecks, and runs src under dash with the working
// directory set to a fresh temp dir (hermetic file I/O). env carries extra
// "NAME=VALUE" entries appended to the process environment.
func runWispDir(t *testing.T, src string, env []string) (string, string, int) {
	t.Helper()
	script := compile(t, src)
	shellcheck(t, script)
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dash, path)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return out.String(), errb.String(), code
}

// --- env / has_env ---

func TestM7_Env_Set(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(env.get("WISP_TEST_VAR"))
  return 0
}`, []string{"WISP_TEST_VAR=hello"}, "env")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "hello\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM7_Env_SetEmpty(t *testing.T) {
	// A set-but-empty variable: env returns "", has_env returns true.
	out, errb, code := runNSDir(t, `fn main() -> int {
  print("[${env.get("WISP_EMPTY")}]")
  print(to_string(env.has("WISP_EMPTY")))
  return 0
}`, []string{"WISP_EMPTY="}, "env")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "[]\ntrue\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM7_Env_Unset_AbortsLoud(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(env.get("WISP_NOPE_XYZ"))
  return 0
}`, nil, "env")
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (out=%q stderr=%q)", code, out, errb)
	}
	if !strings.Contains(errb, "WISP_NOPE_XYZ") || !strings.Contains(errb, "not set") {
		t.Errorf("stderr=%q, want it to name the variable", errb)
	}
}

func TestM7_HasEnv_Unset(t *testing.T) {
	out, _, code := runNSDir(t, `fn main() -> int {
  print(to_string(env.has("WISP_NOPE_XYZ")))
  return 0
}`, nil, "env")
	if code != 0 || out != "false\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_Env_ShellActiveValueInert(t *testing.T) {
	const inj = "$(echo PWN); `id`"
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(env.get("WISP_INJ"))
  return 0
}`, []string{"WISP_INJ=" + inj}, "env")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != inj+"\n" {
		t.Errorf("out=%q (value must be inert)", out)
	}
}

// --- read_file / write_file / append_file ---

func TestM7_File_RoundTripExactBytes(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  fs.write_file("f", "abc\n\n\n")
  let c: string = fs.read_file("f")
  print("[${c}]")
  return 0
}`, nil, "fs")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	// Multiple trailing newlines must be preserved exactly.
	if out != "[abc\n\n\n]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM7_File_WriteNoAddedNewline(t *testing.T) {
	out, _, code := runNSDir(t, `fn main() -> int {
  fs.write_file("f", "x")
  let c: string = fs.read_file("f")
  print("[${c}]")
  return 0
}`, nil, "fs")
	if code != 0 || out != "[x]\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_File_Append(t *testing.T) {
	out, _, code := runNSDir(t, `fn main() -> int {
  fs.write_file("f", "a")
  fs.append_file("f", "b")
  fs.append_file("f", "c")
  print(fs.read_file("f"))
  return 0
}`, nil, "fs")
	if code != 0 || out != "abc\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_File_AppendCreatesAbsent(t *testing.T) {
	out, _, code := runNSDir(t, `fn main() -> int {
  fs.append_file("new", "z")
  print(fs.read_file("new"))
  return 0
}`, nil, "fs")
	if code != 0 || out != "z\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_ReadFile_Missing_AbortsLoud(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(fs.read_file("nope"))
  return 0
}`, nil, "fs")
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (out=%q)", code, out)
	}
	if !strings.Contains(errb, "nope") {
		t.Errorf("stderr=%q, want it to name the path", errb)
	}
}

// --- run ---

func TestM7_Run_Echo(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(process.run(["echo", "hi"]))
  return 0
}`, nil, "process")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	// echo adds a trailing newline; run strips it; print re-adds one.
	if out != "hi\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM7_Run_InteriorNewlinesPreserved(t *testing.T) {
	out, _, code := runNSDir(t, `fn main() -> int {
  print("[${process.run(["printf", "a\nb\n\n"])}]")
  return 0
}`, nil, "process")
	// Interior newline preserved, trailing stripped.
	if code != 0 || out != "[a\nb]\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_Run_NonzeroExit_AbortsLoud(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print(process.run(["false"]))
  return 0
}`, nil, "process")
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (out=%q)", code, out)
	}
	if !strings.Contains(errb, "false") || !strings.Contains(errb, "status 1") {
		t.Errorf("stderr=%q, want argv[0] and the status", errb)
	}
}

func TestM7_Run_CallerParamsIntact(t *testing.T) {
	// run rebuilds argv into the helper's own positionals; the calling function's
	// parameters must survive.
	out, _, code := runNSDir(t, `fn doit(a: string, b: string) -> string {
  let r: string = process.run(["echo", "x"])
  return a + b
}
fn main() -> int {
  print(doit("p", "q"))
  return 0
}`, nil, "process")
	if code != 0 || out != "pq\n" {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestM7_Run_PathResolvesExternalCommand(t *testing.T) {
	// cat is an EXTERNAL command (resolved via PATH), unlike echo which the shell
	// resolves as a builtin first.
	out, errb, code := runNSDir(t, `fn main() -> int {
  fs.write_file("data", "filecontents")
  print(process.run(["cat", "data"]))
  return 0
}`, nil, "fs", "process")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "filecontents\n" {
		t.Errorf("out=%q", out)
	}
}

// --- exit ---

func TestM7_Exit_Status(t *testing.T) {
	out, _, code := runWispDir(t, `fn main() -> int {
  print("before")
  exit(7)
  return 0
}`, nil)
	if code != 7 {
		t.Errorf("exit=%d, want 7", code)
	}
	if out != "before\n" {
		t.Errorf("out=%q", out)
	}
}

// --- tree-shaking ---

func TestM7_TreeShaken_WhenUnused(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  print("hi")
  return 0
}`))
	for _, h := range []string{"__wisp_env", "__wisp_has_env", "__wisp_read_file", "__wisp_write_file", "__wisp_append_file", "__wisp_run"} {
		if strings.Contains(script, h+"()") {
			t.Errorf("unused helper %s present in output", h)
		}
	}
}

func TestM7_Exit_DirectLowering(t *testing.T) {
	// exit lowers directly to `exit <code>`, no helper.
	script := string(compile(t, `fn main() -> int {
  exit(3)
  return 0
}`))
	if !strings.Contains(script, "exit 3") {
		t.Errorf("exit not lowered directly:\n%s", script)
	}
}
