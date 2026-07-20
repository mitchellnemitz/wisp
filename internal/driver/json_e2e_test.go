package driver

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// End-to-end tests for the json core module: full pipeline (module loader ->
// linked check -> codegen) then run under dash. `import "json"` is a reserved
// core import, so no filesystem module is needed.

func runJSONProg(t *testing.T, src string) (string, string, int) {
	t.Helper()
	script, _, diags := Compile("j.wisp", src)
	for _, d := range diags {
		if d.Severity == Error {
			t.Fatalf("compile error: %s: %s", d.Pos, d.Msg)
		}
	}
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
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	code := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return out.String(), errb.String(), code
}

func TestJSONE2EConstructorsEncode(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  print(json.encode(json.from_int(42)))
  print(json.encode(json.from_string("he\"llo")))
  print(json.encode(json.from_bool(true)))
  print(json.encode(json.from_float(1.5)))
  print(json.encode(json.null()))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "42\n\"he\\\"llo\"\ntrue\n1.5\nnull\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

func TestJSONE2EArrayObject(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  let a: json.Value[] = [json.from_int(1), json.from_int(2)]
  print(json.encode(json.array(a)))
  let d: {string: json.Value} = {"x": json.from_int(1), "y": json.from_bool(false)}
  print(json.encode(json.object(d)))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "[1,2]\n{\"x\":1,\"y\":false}\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

func TestJSONE2EDecodeRoundtrip(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  let v: json.Value = json.decode("{ \"a\": [1, 2, 3] }")
  print(json.encode(v))
  print(to_string(json.decode[int]("42")))
  print(to_string(json.decode[float]("1.5")))
  print(to_string(json.decode[bool]("true")))
  print(json.decode[string]("\"hi\""))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "{\"a\":[1,2,3]}\n42\n1.5\ntrue\nhi\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

func TestJSONE2EGetAtOptional(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  let v: json.Value = json.decode("{\"a\": 1, \"b\": [10, 20]}")
  print(json.encode(unwrap(json.get(v, "a"))))
  print(to_string(is_none(json.get(v, "missing"))))
  let arr: json.Value = unwrap(json.get(v, "b"))
  print(json.encode(unwrap(json.at(arr, 1))))
  print(to_string(is_none(json.at(arr, 5))))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "1\ntrue\n20\ntrue\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

func TestJSONE2EAccessorsAndType(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  let v: json.Value = json.decode("{\"s\":\"x\",\"i\":7,\"f\":2.5,\"b\":true}")
  print(json.type_of(v))
  print(json.as_string(unwrap(json.get(v, "s"))))
  print(to_string(json.as_int(unwrap(json.get(v, "i")))))
  print(to_string(json.as_float(unwrap(json.get(v, "f")))))
  print(to_string(json.as_bool(unwrap(json.get(v, "b")))))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "object\nx\n7\n2.5\ntrue\n"
	if out != want {
		t.Errorf("out=%q want=%q", out, want)
	}
}

func TestJSONE2EDebug(t *testing.T) {
	out, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  print(debug(json.decode("[1, 2, {\"k\": true}]")))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "[1,2,{\"k\":true}]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestJSONE2EDecodeMalformedAborts(t *testing.T) {
	_, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  print(json.encode(json.decode("not json")))
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (stderr=%q)", code, errb)
	}
	if !strings.Contains(errb, "invalid JSON") {
		t.Errorf("stderr=%q, want it to mention invalid JSON", errb)
	}
}

func TestJSONE2EInjectionInert(t *testing.T) {
	// Shell-active bytes in JSON data ($(...), ${...}) must be inert: every value
	// flows through double-quoted expansions / ENVIRON, never eval or a format
	// string. The payload round-trips literally; nothing is executed.
	src := "import \"json\"\n" +
		"fn main() -> int {\n" +
		"  let v: json.Value = json.decode(\"{\\\"x\\\": \\\"a $(echo INJ) b\\\"}\")\n" +
		"  print(json.as_string(unwrap(json.get(v, \"x\"))))\n" +
		"  print(json.encode(json.from_string(\"$(echo Z)end\")))\n" +
		"  return 0\n}"
	out, errb, code := runJSONProg(t, src)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "a $(echo INJ) b\n\"$(echo Z)end\"\n"
	if out != want {
		t.Errorf("injection not inert:\n got=%q\nwant=%q", out, want)
	}
}

func TestJSONE2EAsIntWrongTypeAborts(t *testing.T) {
	_, errb, code := runJSONProg(t, `import "json"
fn main() -> int {
  print(to_string(json.as_int(json.from_string("x"))))
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (stderr=%q)", code, errb)
	}
	if !strings.Contains(errb, "not an integer") {
		t.Errorf("stderr=%q", errb)
	}
}
