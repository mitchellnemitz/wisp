package runtime

import (
	"strconv"
	"strings"
	"testing"
)

// shq single-quotes a value for a POSIX sh command line, escaping embedded
// single quotes so arbitrary JSON (including quotes and backslashes) survives.
func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runJSONEngine invokes __wisp_json_awk <op> <in> <arg> under dash and returns
// the sentinel-stripped payload and the awk exit code. The driver itself always
// exits 0; the engine's rc is reported inline so a nonzero (malformed) run is
// observable without the snippet aborting.
func runJSONEngine(t *testing.T, op, in, arg string) (payload string, rc int) {
	t.Helper()
	driver := "SENT=$(printf '\\001')\n" +
		"out=$(__wisp_json_awk " + shq(op) + " " + shq(in) + " " + shq(arg) + "); rc=$?\n" +
		`printf 'RC=%d\n' "$rc"` + "\n" +
		`printf '%s' "${out%"$SENT"}"`
	stdout, stderr, _ := runSnippet(t, []string{JSONEngine}, driver)
	nl := strings.IndexByte(stdout, '\n')
	if nl < 0 {
		t.Fatalf("engine driver produced no RC line: stdout=%q stderr=%q", stdout, stderr)
	}
	rc, err := strconv.Atoi(strings.TrimPrefix(stdout[:nl], "RC="))
	if err != nil {
		t.Fatalf("bad RC line %q: %v", stdout[:nl], err)
	}
	return stdout[nl+1:], rc
}

func TestJSONEngineCanonicalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{`42`, `42`},
		{`-7`, `-7`},
		{`true`, `true`},
		{`false`, `false`},
		{`null`, `null`},
		{`"hi"`, `"hi"`},
		{`  42  `, `42`},
		{`{ "a" : 1 , "b" : 2 }`, `{"a":1,"b":2}`},
		{`[ 1 , 2 , 3 ]`, `[1,2,3]`},
		{`{}`, `{}`},
		{`[]`, `[]`},
		{"{\n\t\"k\": [true, null, {\"n\": -3.5}]\n}", `{"k":[true,null,{"n":-3.5}]}`},
		{`{"a":{"b":{"c":[1]}}}`, `{"a":{"b":{"c":[1]}}}`},
		// Duplicate keys preserved verbatim.
		{`{"a":1,"a":2}`, `{"a":1,"a":2}`},
	}
	for _, c := range cases {
		got, rc := runJSONEngine(t, "validate", c.in, "")
		if rc != 0 {
			t.Errorf("validate(%q): rc=%d, want 0", c.in, rc)
			continue
		}
		if got != c.want {
			t.Errorf("validate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSONEngineNumbersVerbatim(t *testing.T) {
	// Numbers are stored byte-for-byte: no %.17g, no precision loss.
	cases := []string{
		`0.1`,
		`9007199254740993`,
		`1e400`,
		`-0`,
		`1.0`,
		`3.14159265358979323846`,
		`1E+30`,
		`-2.5e-10`,
	}
	for _, in := range cases {
		got, rc := runJSONEngine(t, "validate", in, "")
		if rc != 0 {
			t.Errorf("validate(%q): rc=%d, want 0", in, rc)
			continue
		}
		if got != in {
			t.Errorf("validate(%q) = %q, want verbatim %q", in, got, in)
		}
	}
}

func TestJSONEngineStringsVerbatim(t *testing.T) {
	// String bodies pass through verbatim after escape validation: \u is NOT
	// decoded, valid escapes preserved, UTF-8 bytes preserved.
	cases := []string{
		`"abc"`,
		`"tab\tnl\nquote\"slash\\end"`,
		`"😀"`,             // surrogate pair, preserved verbatim
		"\"caf\xc3\xa9\"", // raw UTF-8 bytes
		`""`,
		`"\/"`,
	}
	for _, in := range cases {
		got, rc := runJSONEngine(t, "validate", in, "")
		if rc != 0 {
			t.Errorf("validate(%q): rc=%d, want 0", in, rc)
			continue
		}
		if got != in {
			t.Errorf("validate(%q) = %q, want verbatim %q", in, got, in)
		}
	}
}

func TestJSONEngineMalformed(t *testing.T) {
	cases := []string{
		``,              // empty
		`   `,           // whitespace only
		`truex`,         // trailing content
		`[1,2`,          // unterminated array
		`{"a":1`,        // unterminated object
		`{"a":1,}`,      // trailing comma in object
		`[1,]`,          // trailing comma in array
		`{"a" 1}`,       // missing colon
		`{a:1}`,         // unquoted key
		`01`,            // leading zero
		`-`,             // bare minus
		`1.`,            // trailing dot
		`.5`,            // leading dot
		`+3`,            // leading plus
		`1e`,            // bare exponent
		`NaN`,           // not a literal
		`Infinity`,      // not a literal
		`"unterminated`, // unterminated string
		`"bad\x"`,       // invalid escape
		`"\uD800"`,      // lone high surrogate
		`"\uDC00"`,      // lone low surrogate
		`"\uD800A"`,     // high surrogate not followed by low
		"\"raw\tctrl\"", // raw control byte in string
		`[1 2]`,         // missing comma
		`}`,             // stray close
		`[}`,            // mismatched close
	}
	for _, in := range cases {
		got, rc := runJSONEngine(t, "validate", in, "")
		if rc == 0 {
			t.Errorf("validate(%q): rc=0 (accepted), want nonzero; payload=%q", in, got)
		}
	}
}

func TestJSONEngineType(t *testing.T) {
	cases := []struct{ in, want string }{
		{`null`, `null`},
		{`true`, `bool`},
		{`false`, `bool`},
		{`42`, `number`},
		{`-3.5`, `number`},
		{`"s"`, `string`},
		{`[1,2]`, `array`},
		{`{"a":1}`, `object`},
	}
	for _, c := range cases {
		got, rc := runJSONEngine(t, "type", c.in, "")
		if rc != 0 {
			t.Errorf("type(%q): rc=%d, want 0", c.in, rc)
			continue
		}
		if got != c.want {
			t.Errorf("type(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestJSONEngineLocale asserts the load-bearing LC_ALL=C on the awk invocation:
// byte-accurate substr and %c depend on it.
func TestJSONEngineLocale(t *testing.T) {
	src := registry[JSONEngine].src
	if !strings.Contains(src, "LC_ALL=C awk") {
		t.Fatal("json engine must run awk under LC_ALL=C")
	}
}

// parseGetAt splits a get/at payload: leading '1'<value> (found) or '0' (absent).
func parseGetAt(payload string) (found bool, value string) {
	if len(payload) == 0 {
		return false, ""
	}
	if payload[0] == '1' {
		return true, payload[1:]
	}
	return false, ""
}

func TestJSONEngineGet(t *testing.T) {
	obj := `{"a":1,"b":"two","c":[1,2],"d":{"e":true},"dup":1,"dup":2}`
	cases := []struct {
		key       string
		wantFound bool
		wantVal   string
	}{
		{"a", true, "1"},
		{"b", true, `"two"`},
		{"c", true, "[1,2]"},
		{"d", true, `{"e":true}`},
		{"dup", true, "1"}, // first occurrence
		{"missing", false, ""},
	}
	for _, c := range cases {
		payload, rc := runJSONEngine(t, "get", obj, c.key)
		if rc != 0 {
			t.Errorf("get(%q): rc=%d, want 0", c.key, rc)
			continue
		}
		found, val := parseGetAt(payload)
		if found != c.wantFound || (found && val != c.wantVal) {
			t.Errorf("get(%q) = (found=%v,%q), want (found=%v,%q)", c.key, found, val, c.wantFound, c.wantVal)
		}
	}
}

func TestJSONEngineGetEscapedKey(t *testing.T) {
	// Key comparison is by DECODED bytes: the JSON key "b" decodes to "b",
	// so a request for "b" must match it and return the verbatim value.
	payload, rc := runJSONEngine(t, "get", `{"b":42}`, "b")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	found, val := parseGetAt(payload)
	if !found || val != "42" {
		t.Fatalf("get decoded-key = (%v,%q), want (true,42)", found, val)
	}
	// A \u-escaped key exercises the UTF-8 decoder: "é" decodes to the two
	// bytes 0xC3 0xA9, matching a request for those raw bytes.
	payload, rc = runJSONEngine(t, "get", `{"\u00e9":7}`, "\xc3\xa9")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if found, val := parseGetAt(payload); !found || val != "7" {
		t.Fatalf("get utf8-key = (%v,%q), want (true,7)", found, val)
	}
}

func TestJSONEngineGetNonObject(t *testing.T) {
	for _, in := range []string{`[1,2]`, `42`, `"s"`, `true`, `null`} {
		payload, rc := runJSONEngine(t, "get", in, "a")
		if rc != 0 {
			t.Errorf("get(%q): rc=%d, want 0", in, rc)
			continue
		}
		if found, _ := parseGetAt(payload); found {
			t.Errorf("get(%q,a) reported found on non-object", in)
		}
	}
}

func TestJSONEngineAt(t *testing.T) {
	arr := `[10,"x",[1,2],{"k":3},true]`
	cases := []struct {
		idx       string
		wantFound bool
		wantVal   string
	}{
		{"0", true, "10"},
		{"1", true, `"x"`},
		{"2", true, "[1,2]"},
		{"3", true, `{"k":3}`},
		{"4", true, "true"},
		{"5", false, ""},
		{"-1", false, ""},
	}
	for _, c := range cases {
		payload, rc := runJSONEngine(t, "at", arr, c.idx)
		if rc != 0 {
			t.Errorf("at(%q): rc=%d, want 0", c.idx, rc)
			continue
		}
		found, val := parseGetAt(payload)
		if found != c.wantFound || (found && val != c.wantVal) {
			t.Errorf("at(%q) = (found=%v,%q), want (found=%v,%q)", c.idx, found, val, c.wantFound, c.wantVal)
		}
	}
}

func TestJSONEngineAtNonArray(t *testing.T) {
	for _, in := range []string{`{"a":1}`, `42`, `"s"`, `null`} {
		payload, rc := runJSONEngine(t, "at", in, "0")
		if rc != 0 {
			t.Errorf("at(%q): rc=%d, want 0", in, rc)
			continue
		}
		if found, _ := parseGetAt(payload); found {
			t.Errorf("at(%q,0) reported found on non-array", in)
		}
	}
}

func TestJSONEngineScalarString(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"abc"`, "abc"},
		{`""`, ""},
		{`"a\nb"`, "a\nb"}, // trailing/embedded newline survives the sentinel
		{`"tab\there"`, "tab\there"},
		{`"q\"q"`, `q"q`},
		{`"back\\slash"`, `back\slash`},
		{`"sl\/ash"`, "sl/ash"},
		{`"A"`, "A"},                       // BMP ascii
		{`"é"`, "\xc3\xa9"},                // é -> 2-byte UTF-8
		{`"€"`, "\xe2\x82\xac"},            // euro -> 3-byte UTF-8
		{`"😀"`, "\xf0\x9f\x98\x80"},        // surrogate pair -> 4-byte UTF-8
		{"\"caf\xc3\xa9\"", "caf\xc3\xa9"}, // raw UTF-8 passthrough
		{`"end\n"`, "end\n"},
	}
	for _, c := range cases {
		got, rc := runJSONEngine(t, "scalar_string", c.in, "")
		if rc != 0 {
			t.Errorf("scalar_string(%q): rc=%d, want 0", c.in, rc)
			continue
		}
		if got != c.want {
			t.Errorf("scalar_string(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJSONEngineScalarStringRejectsNonString(t *testing.T) {
	for _, in := range []string{`42`, `true`, `null`, `[1]`, `{"a":1}`} {
		if _, rc := runJSONEngine(t, "scalar_string", in, ""); rc == 0 {
			t.Errorf("scalar_string(%q): rc=0, want nonzero", in)
		}
	}
}

func TestJSONEngineScalarInt(t *testing.T) {
	for _, in := range []string{`0`, `-0`, `42`, `-7`, `9007199254740993`} {
		got, rc := runJSONEngine(t, "scalar_int", in, "")
		if rc != 0 {
			t.Errorf("scalar_int(%q): rc=%d, want 0", in, rc)
			continue
		}
		if got != in {
			t.Errorf("scalar_int(%q) = %q, want verbatim", in, got)
		}
	}
	// Floats, exponents, and non-numbers are rejected (a located abort in the wrapper).
	for _, in := range []string{`1.5`, `1e5`, `1.0`, `"5"`, `true`, `[1]`, `01`} {
		if _, rc := runJSONEngine(t, "scalar_int", in, ""); rc == 0 {
			t.Errorf("scalar_int(%q): rc=0, want nonzero", in)
		}
	}
}

func TestJSONEngineScalarFloat(t *testing.T) {
	for _, in := range []string{`1.5`, `-2.5e-10`, `42`, `0.1`, `1e400`} {
		got, rc := runJSONEngine(t, "scalar_float", in, "")
		if rc != 0 {
			t.Errorf("scalar_float(%q): rc=%d, want 0", in, rc)
			continue
		}
		if got != in {
			t.Errorf("scalar_float(%q) = %q, want verbatim token", in, got)
		}
	}
	for _, in := range []string{`"x"`, `true`, `null`, `[1]`, `{"a":1}`} {
		if _, rc := runJSONEngine(t, "scalar_float", in, ""); rc == 0 {
			t.Errorf("scalar_float(%q): rc=0, want nonzero", in)
		}
	}
}

func TestJSONEngineScalarBool(t *testing.T) {
	for _, c := range []struct{ in, want string }{{`true`, `true`}, {`false`, `false`}} {
		got, rc := runJSONEngine(t, "scalar_bool", c.in, "")
		if rc != 0 || got != c.want {
			t.Errorf("scalar_bool(%q) = (%q,rc=%d), want (%q,0)", c.in, got, rc, c.want)
		}
	}
	for _, in := range []string{`1`, `"true"`, `null`, `[true]`} {
		if _, rc := runJSONEngine(t, "scalar_bool", in, ""); rc == 0 {
			t.Errorf("scalar_bool(%q): rc=0, want nonzero", in)
		}
	}
}

// runJSONWrapper runs a driver that calls a json wrapper and prints __ret, plus
// the wrapper's abort status (exit code). Returns stdout (ret), stderr, code.
func runJSONWrapper(t *testing.T, helpers []string, driver string) (string, string, int) {
	t.Helper()
	return runSnippet(t, helpers, driver)
}

func TestJSONWrapperValidateAndEscape(t *testing.T) {
	out, _, code := runJSONWrapper(t, []string{JSONValidate, JSONEscape},
		`__wisp_json_validate "p:1:1" '{ "a" : 1 }'; printf 'V=%s\n' "$__ret"; `+
			`__wisp_json_escape 'a"b\c'; printf 'E=%s\n' "$__ret"`)
	if code != 0 {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if !strings.Contains(out, `V={"a":1}`) {
		t.Errorf("validate: %q", out)
	}
	if !strings.Contains(out, `E="a\"b\\c"`) {
		t.Errorf("escape: %q", out)
	}
}

func TestJSONWrapperValidateAborts(t *testing.T) {
	_, errb, code := runJSONWrapper(t, []string{JSONValidate},
		`__wisp_json_validate "prog.wisp:2:5" 'not json'`)
	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if !strings.Contains(errb, "prog.wisp:2:5") || !strings.Contains(errb, "invalid JSON") {
		t.Errorf("stderr=%q", errb)
	}
}

func TestJSONWrapperDecodeScalars(t *testing.T) {
	out, _, code := runJSONWrapper(t, []string{JSONDecodeInt, JSONDecodeFloat, JSONDecodeBool, JSONDecodeString},
		`__wisp_json_decode_int "p:1:1" '42'; printf 'I=%s\n' "$__ret"; `+
			`__wisp_json_decode_float "p:1:1" '1.5'; printf 'F=%s\n' "$__ret"; `+
			`__wisp_json_decode_bool "p:1:1" 'true'; printf 'B=%s\n' "$__ret"; `+
			`__wisp_json_decode_string "p:1:1" '"hi\n"'; printf 'S=%s.\n' "$__ret"`)
	if code != 0 {
		t.Fatalf("code=%d out=%q", code, out)
	}
	for _, want := range []string{"I=42", "F=1.5", "B=true", "S=hi\n."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestJSONWrapperDecodeIntWrongType(t *testing.T) {
	_, errb, code := runJSONWrapper(t, []string{JSONDecodeInt},
		`__wisp_json_decode_int "prog.wisp:3:9" '"nope"'`)
	if code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if !strings.Contains(errb, "not an integer") {
		t.Errorf("stderr=%q", errb)
	}
}

func TestJSONWrapperGetAt(t *testing.T) {
	out, _, code := runJSONWrapper(t, []string{JSONGet, JSONAt},
		`__wisp_json_get "p:1:1" '{"a":1,"b":[10,20]}' 'b'; printf 'G=%s\n' "$__ret"; `+
			`__wisp_json_at "p:1:1" '[10,20]' '1'; printf 'A=%s\n' "$__ret"; `+
			`__wisp_json_get "p:1:1" '{"a":1}' 'zzz'; printf 'M=%s\n' "$__ret"`)
	if code != 0 {
		t.Fatalf("code=%d out=%q", code, out)
	}
	for _, want := range []string{"G=1[10,20]", "A=120", "M=0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}
