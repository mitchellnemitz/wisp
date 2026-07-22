package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/format"
)

// --- framing / driving helpers ---

func reqFrame(id int, method, params string) string {
	if params == "" {
		params = "null"
	}
	body := `{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"` + method + `","params":` + params + `}`
	return frame(body)
}

func notifFrame(method, params string) string {
	if params == "" {
		params = "null"
	}
	return frame(`{"jsonrpc":"2.0","method":"` + method + `","params":` + params + `}`)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func didOpenFrame(uri, text string) string {
	return notifFrame("textDocument/didOpen", mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "wisp", "version": 1, "text": text},
	}))
}

func didChangeFrame(uri, text string) string {
	return notifFrame("textDocument/didChange", mustJSON(map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []map[string]any{{"text": text}},
	}))
}

func didCloseFrame(uri string) string {
	return notifFrame("textDocument/didClose", mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}))
}

func docReqFrame(id int, method, uri string) string {
	return reqFrame(id, method, mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}))
}

func posReqFrame(id int, method, uri string, line, char int) string {
	return reqFrame(id, method, mustJSON(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
	}))
}

func runServer(input string) (int, []map[string]any) {
	var out bytes.Buffer
	code := Serve(strings.NewReader(input), &out, io.Discard)
	return code, decodeFrames(out.Bytes())
}

func decodeFrames(data []byte) []map[string]any {
	var msgs []map[string]any
	r := bufio.NewReader(bytes.NewReader(data))
	for {
		cl := -1
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return msgs
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				v := strings.TrimSpace(line[len("content-length:"):])
				cl = 0
				for _, c := range v {
					cl = cl*10 + int(c-'0')
				}
			}
		}
		if cl < 0 {
			return msgs
		}
		body := make([]byte, cl)
		if _, err := io.ReadFull(r, body); err != nil {
			return msgs
		}
		var m map[string]any
		if json.Unmarshal(body, &m) == nil {
			msgs = append(msgs, m)
		}
	}
}

func respForID(msgs []map[string]any, id int) map[string]any {
	for _, m := range msgs {
		if v, ok := m["id"]; ok {
			if f, ok := v.(float64); ok && int(f) == id {
				return m
			}
		}
	}
	return nil
}

// lastPublishFor returns the diagnostics array from the LAST
// publishDiagnostics notification for uri.
func lastPublishFor(msgs []map[string]any, uri string) []any {
	var diags []any
	found := false
	for _, m := range msgs {
		if m["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		p, _ := m["params"].(map[string]any)
		if p == nil || p["uri"] != uri {
			continue
		}
		d, _ := p["diagnostics"].([]any)
		diags = d
		found = true
	}
	if !found {
		return nil
	}
	if diags == nil {
		return []any{}
	}
	return diags
}

const uri = "file:///a.wisp"

// --- tests ---

func TestInitializeCapabilities(t *testing.T) {
	_, msgs := runServer(reqFrame(1, "initialize", "{}"))
	resp := respForID(msgs, 1)
	if resp == nil {
		t.Fatal("no initialize response")
	}
	result, _ := resp["result"].(map[string]any)
	caps, _ := result["capabilities"].(map[string]any)
	if caps == nil {
		t.Fatalf("no capabilities in %v", resp)
	}
	// MUST floor.
	if caps["documentFormattingProvider"] != true {
		t.Error("documentFormattingProvider not advertised")
	}
	if caps["documentSymbolProvider"] != true {
		t.Error("documentSymbolProvider not advertised")
	}
	tds, _ := caps["textDocumentSync"].(map[string]any)
	if tds == nil || tds["change"] != float64(1) {
		t.Errorf("textDocumentSync = %v, want change=1 (Full)", caps["textDocumentSync"])
	}
	// SHOULD features: implemented in A3, so they MUST be advertised here
	// (advertise iff implemented).
	if caps["hoverProvider"] != true {
		t.Error("hoverProvider implemented but not advertised")
	}
	if _, ok := caps["completionProvider"]; !ok {
		t.Error("completionProvider implemented but not advertised")
	}
}

func TestDiagnosticsTypeErrorThenFixedToEmpty(t *testing.T) {
	typeErr := "fn main() -> int {\n  let x: int = \"hello\"\n  return 0\n}\n"
	clean := "fn add(a: int, b: int) -> int {\n  return a + b\n}\nfn main() -> int {\n  return add(1, 2)\n}\n"

	// Open with a type error.
	_, msgs := runServer(didOpenFrame(uri, typeErr))
	diags := lastPublishFor(msgs, uri)
	var errDiag map[string]any
	for _, d := range diags {
		dm := d.(map[string]any)
		if dm["severity"] == float64(severityError) {
			errDiag = dm
		}
	}
	if errDiag == nil {
		t.Fatalf("expected an Error diagnostic, got %v", diags)
	}
	rng := errDiag["range"].(map[string]any)
	start := rng["start"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(15) {
		t.Errorf("error range start = %v, want line 1 char 15", start)
	}

	// Change to a clean program -> diagnostics cleared to empty.
	_, msgs = runServer(didOpenFrame(uri, typeErr) + didChangeFrame(uri, clean))
	diags = lastPublishFor(uri2msgs(msgs), uri)
	if len(diags) != 0 {
		t.Errorf("after fix, diagnostics = %v, want empty", diags)
	}
}

// uri2msgs is a no-op kept for clarity in the change test above.
func uri2msgs(m []map[string]any) []map[string]any { return m }

func TestDiagnosticsWarningOnly(t *testing.T) {
	warn := "fn main() -> int {\n  let x: int = 1\n  return 0\n}\n" // unused x -> warning
	_, msgs := runServer(didOpenFrame(uri, warn))
	diags := lastPublishFor(msgs, uri)
	if len(diags) != 1 {
		t.Fatalf("want exactly one diagnostic, got %v", diags)
	}
	if diags[0].(map[string]any)["severity"] != float64(severityWarning) {
		t.Errorf("severity = %v, want Warning(2)", diags[0])
	}
}

func TestDiagnosticsParseErrorIsSingle(t *testing.T) {
	bad := "fn main() -> int {\n  let x: int =\n}\n"
	_, msgs := runServer(didOpenFrame(uri, bad))
	diags := lastPublishFor(msgs, uri)
	if len(diags) != 1 {
		t.Fatalf("parse error: want exactly one diagnostic, got %v", diags)
	}
	if diags[0].(map[string]any)["severity"] != float64(severityError) {
		t.Errorf("parse-error severity = %v, want Error(1)", diags[0])
	}
}

func TestDiagnosticsMultibytePosition(t *testing.T) {
	// The é (2 bytes / 1 UTF-16 unit) precedes the erroring token on line 2, so
	// the byte column (37) differs from the UTF-16 character (35).
	src := "fn main() -> int {\n" +
		"  let x: int = length(\"héllo\") + 1 + \"z\"\n" +
		"  return 0\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src))
	diags := lastPublishFor(msgs, uri)
	var errDiag map[string]any
	for _, d := range diags {
		dm := d.(map[string]any)
		if dm["severity"] == float64(severityError) {
			errDiag = dm
		}
	}
	if errDiag == nil {
		t.Fatalf("expected an Error diagnostic, got %v", diags)
	}
	start := errDiag["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(1) || start["character"] != float64(35) {
		t.Errorf("multibyte error start = %v, want line 1 char 35 (byte col was 37)", start)
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	warn := "fn main() -> int {\n  let x: int = 1\n  return 0\n}\n"
	_, msgs := runServer(didOpenFrame(uri, warn) + didCloseFrame(uri))
	diags := lastPublishFor(msgs, uri)
	if diags == nil {
		t.Fatal("no publishDiagnostics seen for close")
	}
	if len(diags) != 0 {
		t.Errorf("didClose diagnostics = %v, want empty", diags)
	}
}

func TestFormattingProducesEditEqualToFormat(t *testing.T) {
	src := "fn main()->int {\nreturn 0\n}\n" // valid but not canonical
	want, err := format.Format(src, "")
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if want == src {
		t.Fatal("fixture is already canonical; pick an unformatted one")
	}
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/formatting", uri))
	resp := respForID(msgs, 2)
	edits, _ := resp["result"].([]any)
	if len(edits) != 1 {
		t.Fatalf("want one TextEdit, got %v", resp["result"])
	}
	e := edits[0].(map[string]any)
	if e["newText"] != want {
		t.Errorf("newText = %q, want %q", e["newText"], want)
	}
	start := e["range"].(map[string]any)["start"].(map[string]any)
	if start["line"] != float64(0) || start["character"] != float64(0) {
		t.Errorf("edit start = %v, want {0,0}", start)
	}
}

func TestFormattingNoEditOnParseError(t *testing.T) {
	bad := "fn main() -> int {\n  let x: int =\n}\n"
	_, msgs := runServer(didOpenFrame(uri, bad) + docReqFrame(2, "textDocument/formatting", uri))
	resp := respForID(msgs, 2)
	edits, _ := resp["result"].([]any)
	if len(edits) != 0 {
		t.Errorf("parse error: want no edits, got %v", edits)
	}
}

func TestDocumentSymbolKindsAndNameRanges(t *testing.T) {
	src := "struct Point {\n  x: int,\n  y: int,\n}\nfn area(p: Point) -> int {\n  return p.x\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	syms, _ := resp["result"].([]any)
	if len(syms) != 2 {
		t.Fatalf("want 2 symbols, got %v", resp["result"])
	}
	point := syms[0].(map[string]any)
	if point["name"] != "Point" || point["kind"] != float64(symbolKindStruct) {
		t.Errorf("symbol 0 = %v, want Point/Struct", point)
	}
	// selectionRange must cover the NAME "Point" (line 0, cols 8..12 -> chars 7..12).
	psel := point["selectionRange"].(map[string]any)
	pstart := psel["start"].(map[string]any)
	pend := psel["end"].(map[string]any)
	if pstart["line"] != float64(0) || pstart["character"] != float64(7) || pend["character"] != float64(12) {
		t.Errorf("Point selectionRange = %v..%v, want char 7..12 on line 0", pstart, pend)
	}

	area := syms[1].(map[string]any)
	if area["name"] != "area" || area["kind"] != float64(symbolKindFunction) {
		t.Errorf("symbol 1 = %v, want area/Function", area)
	}
	// FuncDecl has no NamePos: the name range is derived by scanning past `fn`.
	// "fn area" on line 4 -> name at char 3..7, NOT the `fn` keyword (char 0).
	asel := area["selectionRange"].(map[string]any)
	astart := asel["start"].(map[string]any)
	aend := asel["end"].(map[string]any)
	if astart["line"] != float64(4) || astart["character"] != float64(3) || aend["character"] != float64(7) {
		t.Errorf("area selectionRange = %v..%v, want char 3..7 on line 4 (the name, not `fn`)", astart, aend)
	}
}

func TestHoverAndCompletionEnum(t *testing.T) {
	// `Color` used as a type annotation on line 1; hovering it yields enum hover
	// text. "  let c: " is 9 chars, so `Color` starts at character 9 on line 1.
	src := "enum Color: int { Red, Green, Blue }\n" +
		"fn main() -> int {\n" +
		"  let c: Color = Color.Red\n" +
		"  return to_int(c)\n" +
		"}\n"

	// Hover on the `Color` type annotation (line 2, char 9).
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/hover", uri, 2, 9))
	resp := respForID(msgs, 2)
	if resp == nil {
		t.Fatal("no hover response")
	}
	hv, _ := resp["result"].(map[string]any)
	if hv == nil {
		t.Fatalf("hover result is null, want enum hover text")
	}
	contents, _ := hv["contents"].(map[string]any)
	val, _ := contents["value"].(string)
	if !strings.Contains(val, "enum Color") || !strings.Contains(val, "Red") {
		t.Errorf("hover value = %q, want the enum Color summary", val)
	}

	// Completion includes the enum type name with the enum kind.
	_, msgs = runServer(didOpenFrame(uri, src) + posReqFrame(3, "textDocument/completion", uri, 3, 10))
	resp = respForID(msgs, 3)
	items, _ := resp["result"].([]any)
	if len(items) == 0 {
		t.Fatal("completion returned no items")
	}
	foundEnum := false
	for _, it := range items {
		m := it.(map[string]any)
		if m["label"] == "Color" {
			foundEnum = true
			if m["kind"] != float64(completionKindEnum) {
				t.Errorf("Color completion kind = %v, want enum (%d)", m["kind"], completionKindEnum)
			}
		}
	}
	if !foundEnum {
		t.Error("completion missing enum type \"Color\"")
	}
}

func TestDocumentSymbolEnum(t *testing.T) {
	src := "enum Color: int {\n  Red,\n  Green,\n  Blue,\n}\nfn main() -> int {\n  return 0\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	syms, _ := resp["result"].([]any)
	if len(syms) != 2 {
		t.Fatalf("want 2 symbols, got %v", resp["result"])
	}
	// Enums are emitted after structs and before funcs; here struct is absent so
	// the enum is symbol 0.
	color := syms[0].(map[string]any)
	if color["name"] != "Color" || color["kind"] != float64(symbolKindEnum) {
		t.Errorf("symbol 0 = %v, want Color/Enum", color)
	}
	// selectionRange must cover the NAME "Color" (line 0, cols 6..11 -> chars 5..10).
	csel := color["selectionRange"].(map[string]any)
	cstart := csel["start"].(map[string]any)
	cend := csel["end"].(map[string]any)
	if cstart["line"] != float64(0) || cstart["character"] != float64(5) || cend["character"] != float64(10) {
		t.Errorf("Color selectionRange = %v..%v, want char 5..10 on line 0", cstart, cend)
	}
}

func TestShutdownExitCodes(t *testing.T) {
	// shutdown then exit -> 0.
	code, msgs := runServer(reqFrame(1, "shutdown", "") + notifFrame("exit", ""))
	if code != 0 {
		t.Errorf("shutdown+exit code = %d, want 0", code)
	}
	if resp := respForID(msgs, 1); resp == nil || resp["result"] == nil && resp["error"] != nil {
		// shutdown must return a (null) result, not an error.
		if resp != nil && resp["error"] != nil {
			t.Errorf("shutdown returned error %v", resp["error"])
		}
	}
	// exit without prior shutdown -> 1.
	if code, _ := runServer(notifFrame("exit", "")); code != 1 {
		t.Errorf("exit-without-shutdown code = %d, want 1", code)
	}
}

func TestUnknownMethodAndNotification(t *testing.T) {
	// Unknown request -> method-not-found error.
	_, msgs := runServer(reqFrame(7, "textDocument/bogus", "{}"))
	resp := respForID(msgs, 7)
	if resp == nil || resp["error"] == nil {
		t.Fatalf("unknown request: want error response, got %v", resp)
	}
	if resp["error"].(map[string]any)["code"] != float64(codeMethodNotFound) {
		t.Errorf("error code = %v, want %d", resp["error"], codeMethodNotFound)
	}
	// Unknown notification -> ignored (no output, no crash).
	_, msgs = runServer(notifFrame("textDocument/bogusNotif", "{}"))
	if len(msgs) != 0 {
		t.Errorf("unknown notification produced output: %v", msgs)
	}
}

func TestPanicRecoveredAndServerStillServes(t *testing.T) {
	var out bytes.Buffer
	s := newServer(strings.NewReader(""), &out, io.Discard)

	// A handler that panics, run under the same recovery wrapper handle() uses.
	s.safely(&message{Method: "x", ID: json.RawMessage("9")}, func() { panic("boom") })
	frames := decodeFrames(out.Bytes())
	resp := respForID(frames, 9)
	if resp == nil || resp["error"] == nil {
		t.Fatalf("panic: want internal-error response for id 9, got %v", frames)
	}
	if resp["error"].(map[string]any)["code"] != float64(codeInternalError) {
		t.Errorf("panic error code = %v, want %d", resp["error"], codeInternalError)
	}

	// The server still serves: a normal request after the panic works.
	out.Reset()
	s.dispatch(&message{Method: "initialize", ID: json.RawMessage("10")})
	frames = decodeFrames(out.Bytes())
	if respForID(frames, 10) == nil {
		t.Error("server did not respond to a request after a recovered panic")
	}
}

func TestHoverAndCompletion(t *testing.T) {
	src := "fn add(a: int, b: int) -> int {\n  return a + b\n}\nfn main() -> int {\n  return add(1, 2)\n}\n"

	// Hover on the call to `add` (line 4, inside "add"). "  return " is 9 chars,
	// so `add` starts at character 9.
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/hover", uri, 4, 10))
	resp := respForID(msgs, 2)
	if resp == nil {
		t.Fatal("no hover response")
	}
	hv, _ := resp["result"].(map[string]any)
	if hv == nil {
		t.Fatalf("hover result is null, want a signature")
	}
	contents, _ := hv["contents"].(map[string]any)
	val, _ := contents["value"].(string)
	if !strings.Contains(val, "add") || !strings.Contains(val, "int") {
		t.Errorf("hover value = %q, want the add signature", val)
	}

	// Hover on empty space -> null.
	_, msgs = runServer(didOpenFrame(uri, src) + posReqFrame(3, "textDocument/hover", uri, 1, 0))
	resp = respForID(msgs, 3)
	if resp == nil {
		t.Fatal("no hover response for empty position")
	}
	if _, present := resp["result"]; present && resp["result"] != nil {
		t.Errorf("hover on whitespace = %v, want null", resp["result"])
	}

	// Completion includes keywords + builtins + declared names.
	_, msgs = runServer(didOpenFrame(uri, src) + posReqFrame(4, "textDocument/completion", uri, 4, 10))
	resp = respForID(msgs, 4)
	items, _ := resp["result"].([]any)
	if len(items) == 0 {
		t.Fatal("completion returned no items")
	}
	labels := map[string]bool{}
	for _, it := range items {
		labels[it.(map[string]any)["label"].(string)] = true
	}
	for _, want := range []string{"if", "while", "print", "length", "add", "main", "int", "error"} {
		if !labels[want] {
			t.Errorf("completion missing %q", want)
		}
	}
}

func TestInvalidParamsReturnsError(t *testing.T) {
	// Params that are well-formed JSON but the wrong shape (an array where an
	// object is required) must yield a JSON-RPC -32602 Invalid params error, not
	// a success result computed from a zero-valued struct.
	methods := []string{
		"textDocument/documentSymbol",
		"textDocument/hover",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/rename",
		"textDocument/formatting",
		"textDocument/completion",
		"textDocument/signatureHelp",
		"textDocument/codeAction",
	}
	for _, method := range methods {
		_, msgs := runServer(reqFrame(9, method, "[1,2,3]"))
		resp := respForID(msgs, 9)
		if resp == nil {
			t.Fatalf("%s: no response", method)
		}
		errObj, _ := resp["error"].(map[string]any)
		if errObj == nil {
			t.Fatalf("%s: want error response, got %v", method, resp)
		}
		if errObj["code"] != float64(codeInvalidParams) {
			t.Errorf("%s: error code = %v, want %d", method, errObj["code"], codeInvalidParams)
		}
		if _, hasResult := resp["result"]; hasResult {
			t.Errorf("%s: response carries a result alongside the error", method)
		}
	}
}

func TestWellFormedParamsStillServed(t *testing.T) {
	// A well-formed request still produces a normal result after the invalid-params
	// guard is in place.
	src := "fn main() -> int {\n  return 0\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	if resp == nil || resp["error"] != nil {
		t.Fatalf("well-formed request: want a result, got %v", resp)
	}
	if _, ok := resp["result"].([]any); !ok {
		t.Errorf("want an array result, got %v", resp["result"])
	}
}

func TestNotificationInvalidParamsIgnored(t *testing.T) {
	// A notification with malformed params gets no error reply (per JSON-RPC), is
	// not applied to document state, and is logged rather than silently accepted.
	var out, errBuf bytes.Buffer
	in := notifFrame("textDocument/didOpen", "[1,2,3]") +
		docReqFrame(2, "textDocument/documentSymbol", uri)
	Serve(strings.NewReader(in), &out, &errBuf)
	msgs := decodeFrames(out.Bytes())

	// No error response was emitted for the notification.
	for _, m := range msgs {
		if _, hasErr := m["error"]; hasErr {
			t.Fatalf("notification produced an error response: %v", m)
		}
	}
	// The malformed didOpen did not create doc state, so no diagnostics were
	// published for it.
	for _, m := range msgs {
		if m["method"] == "textDocument/publishDiagnostics" {
			t.Errorf("malformed didOpen still published diagnostics: %v", m)
		}
	}
	// The failure was logged (deliberate handling, not a silent drop).
	if !strings.Contains(errBuf.String(), "didOpen") {
		t.Errorf("expected a log line naming the ignored notification, got %q", errBuf.String())
	}
	// The server stayed aligned and alive: the following request still answers.
	if respForID(msgs, 2) == nil {
		t.Error("server did not answer the request after a malformed notification")
	}
}

func TestDocumentSymbolRangeEndStringBraces(t *testing.T) {
	// A brace inside a string literal in the body must not desync the range-end
	// counter: the declaration range must extend to the real closing brace.
	src := "fn f() -> string {\n  let s: string = \"x }\"\n  return s\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	syms, _ := resp["result"].([]any)
	if len(syms) != 1 {
		t.Fatalf("want 1 symbol, got %v", resp["result"])
	}
	rng := syms[0].(map[string]any)["range"].(map[string]any)
	end := rng["end"].(map[string]any)
	// The real closing brace sits alone on line 3 (0-based).
	if end["line"] != float64(3) {
		t.Errorf("range end line = %v, want 3 (the real closing brace, not the one in the string)", end["line"])
	}
}

func TestDocumentSymbolRangeEndNormalBody(t *testing.T) {
	// A brace-free string body keeps a correct, unchanged range end.
	src := "fn f() -> string {\n  let s: string = \"plain\"\n  return s\n}\n"
	_, msgs := runServer(didOpenFrame(uri, src) + docReqFrame(2, "textDocument/documentSymbol", uri))
	resp := respForID(msgs, 2)
	syms, _ := resp["result"].([]any)
	if len(syms) != 1 {
		t.Fatalf("want 1 symbol, got %v", resp["result"])
	}
	rng := syms[0].(map[string]any)["range"].(map[string]any)
	end := rng["end"].(map[string]any)
	if end["line"] != float64(3) || end["character"] != float64(1) {
		t.Errorf("range end = %v, want line 3 char 1", end)
	}
}

// errorMessages filters diags to just the Error-severity messages. Shared by
// every test in this file and module_test.go that asserts on suppression
// behavior via the direct computeDiagnostics(src) seam.
func errorMessages(diags []lspDiagnostic) []string {
	var msgs []string
	for _, d := range diags {
		if d.Severity == severityError {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs
}

// TestDiagnosticsSuppressUnaliasedCoreImport: an unaliased core-module import
// (e.g. `import "string"`) used as a qualifier must not surface a spurious
// "undeclared name" error -- the bare import path IS the module's namespace
// name, a compiler-known constant, not project state.
func TestDiagnosticsSuppressUnaliasedCoreImport(t *testing.T) {
	src := "import \"string\"\n" +
		"fn main() -> int {\n" +
		"  print(string.trim(\"hi\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("unaliased core import: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsSuppressUnaliasedPackageImport: an unaliased package import
// (e.g. `import "acme/pkgname"`) defaults its namespace to the path's final
// segment, matching the common wisp.json-name convention.
func TestDiagnosticsSuppressUnaliasedPackageImport(t *testing.T) {
	src := "import \"acme/pkgname\"\n" +
		"fn main() -> int {\n" +
		"  print(pkgname.trim(\"hi\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("unaliased package import: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsAliasContinueExcludesBareName: bufferAliases's `continue`
// after recording an explicit `as` alias must exclude the import's bare
// default name -- using the BARE name (not the alias) is still a genuine
// unresolvable reference and must still be reported.
func TestDiagnosticsAliasContinueExcludesBareName(t *testing.T) {
	src := "import \"env\" as e\n" +
		"fn main() -> int {\n" +
		"  print(env.get(\"HOME\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	msgs := errorMessages(diags)
	found := false
	for _, m := range msgs {
		if containsAll(m, "\"env\"", "moved to a module") {
			found = true
		}
	}
	if !found {
		t.Errorf("bare unaliased name with an explicit alias present should still error, got %v", msgs)
	}
}

// TestDiagnosticsIncludeDefaultStemStillSuppressed: the pre-existing
// include-default-stem suppression path (untouched by this fix's Imports-loop
// restructuring) must keep working.
func TestDiagnosticsIncludeDefaultStemStillSuppressed(t *testing.T) {
	src := "include \"./lib/helpers.wisp\"\n" +
		"fn main() -> int {\n" +
		"  print(helpers.trim(\"hi\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("include default stem: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsSuppressAliasedRemovedBuiltinModule: the pre-existing
// explicit-alias path for a removed-builtin-named module must keep working
// after the bufferAliases Imports-loop restructuring. This is a regression
// guard, not a case this task's fix newly repairs: the unresolved identifier
// in `e.get(...)` is literally "e", not "env", so isBuiltin("e") is false
// (internal/types/expr.go:176; builtinSigs has no "e" key) and this already
// hits the ordinary "undeclared name " path, already suppressed pre-fix by
// bufferAliases's existing explicit-alias branch.
func TestDiagnosticsSuppressAliasedRemovedBuiltinModule(t *testing.T) {
	src := "import \"env\" as e\n" +
		"fn main() -> int {\n" +
		"  print(e.get(\"HOME\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("aliased env.get: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsSuppressUnaliasedRemovedBuiltinModule: an unaliased core
// module whose bare name used to be a flat builtin (e.g. `env`) hits a
// DIFFERENT message shape ("was moved to a module") than a plain undeclared
// name -- this must also be suppressed when used correctly and unaliased.
func TestDiagnosticsSuppressUnaliasedRemovedBuiltinModule(t *testing.T) {
	src := "import \"env\"\n" +
		"fn main() -> int {\n" +
		"  print(env.get(\"HOME\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("unaliased env.get: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsSuppressUnaliasedTypeAnnotationNamespace: an unaliased
// import used as a qualified TYPE (not a call) hits the third message shape,
// "unknown namespace ", emitted by resolveNamedType. This shape's diagnostic
// position is never followed by a dot at any reachable call site (the `let`
// keyword's position for a type annotation, or the type name's position
// after the dot for a qualified struct literal) -- suppression must not
// depend on followedByDot for this shape.
func TestDiagnosticsSuppressUnaliasedTypeAnnotationNamespace(t *testing.T) {
	src := "import \"json\"\n" +
		"fn main() -> int {\n" +
		"  let _: json.Value = 0\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	if msgs := errorMessages(diags); len(msgs) != 0 {
		t.Errorf("unaliased json.Value type annotation: want 0 error diagnostics, got %v", msgs)
	}
}

// TestDiagnosticsUndeclaredNameStillReportedWithUnrelatedImportPresent: a
// genuinely undeclared name unrelated to any import must still be reported
// even when the buffer has an unrelated import (a non-empty alias set) --
// proves the widened suppressedForAlias guard doesn't over-suppress. The
// fixture must include an unrelated import: with a fully empty alias set,
// suppressedForAlias short-circuits to false before ever reaching the
// message-shape check, which would make this test pass vacuously.
func TestDiagnosticsUndeclaredNameStillReportedWithUnrelatedImportPresent(t *testing.T) {
	src := "import \"string\"\n" +
		"fn main() -> int {\n" +
		"  let x: int = totallyUndeclaredName\n" +
		"  return x\n" +
		"}\n"
	diags := computeDiagnostics(src)
	msgs := errorMessages(diags)
	found := false
	for _, m := range msgs {
		if containsAll(m, "undeclared name", "\"totallyUndeclaredName\"") {
			found = true
		}
	}
	if !found {
		t.Errorf("genuinely undeclared name should still error, got %v", msgs)
	}
}

// TestDiagnosticsBareAliasValueStillReportedForModuleShape: a known alias
// used as a bare value (no trailing dot) must still be reported even for the
// new "was moved to a module" shape -- proves the trailing-dot gate still
// applies to that shape exactly as it already does for "undeclared name ".
func TestDiagnosticsBareAliasValueStillReportedForModuleShape(t *testing.T) {
	src := "import \"env\"\n" +
		"fn main() -> int {\n" +
		"  let x: int = env\n" +
		"  return x\n" +
		"}\n"
	diags := computeDiagnostics(src)
	msgs := errorMessages(diags)
	found := false
	for _, m := range msgs {
		if containsAll(m, "\"env\"", "moved to a module") {
			found = true
		}
	}
	if !found {
		t.Errorf("bare env value (no trailing dot) should still error, got %v", msgs)
	}
}

// TestDiagnosticsBareFlatCallStillReported: the pre-migration bare flat-call
// form (env("HOME"), not env.get("HOME")) must still be reported. Its
// diagnostic position is always followed by `(`, never `.` -- proves
// followedByDot correctly excludes this second removedHint call site
// (call.go's checkNamedCall) from suppression.
func TestDiagnosticsBareFlatCallStillReported(t *testing.T) {
	src := "import \"env\"\n" +
		"fn main() -> int {\n" +
		"  print(env(\"HOME\"))\n" +
		"  return 0\n" +
		"}\n"
	diags := computeDiagnostics(src)
	msgs := errorMessages(diags)
	found := false
	for _, m := range msgs {
		if containsAll(m, "\"env\"", "moved to a module") {
			found = true
		}
	}
	if !found {
		t.Errorf("bare flat call env(\"HOME\") should still error, got %v", msgs)
	}
}

// hoverValue runs a hover request and returns its rendered value, or ("",
// false) if the result was null.
func hoverValue(src string, line, char int) (string, bool) {
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/hover", uri, line, char))
	resp := respForID(msgs, 2)
	if resp == nil {
		return "", false
	}
	hv, ok := resp["result"].(map[string]any)
	if !ok || hv == nil {
		return "", false
	}
	contents, _ := hv["contents"].(map[string]any)
	val, _ := contents["value"].(string)
	return val, true
}

// TestHoverNamespacedMember covers T-LSP-2: hover on a namespaced core-module
// member in call, funcref, and arg position resolves through the compiler
// catalog and shows its static signature.
func TestHoverNamespacedMember(t *testing.T) {
	src := "import \"string\"\n" +
		"import \"array\"\n" +
		"\n" +
		"fn main() -> void {\n" +
		"  let raw: string = \" hi \"\n" +
		"  string.trim(raw)\n" +
		"  let f: fn(string) -> string = string.trim\n" +
		"  array.map(string.trim, [raw])\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	want := "(builtin) string.trim(a: string) -> string"

	cases := []struct {
		name string
		line int
	}{
		{"call", 5},
		{"funcref", 6},
		{"arg", 7},
	}
	for _, c := range cases {
		col := strings.Index(lines[c.line], "trim")
		if col < 0 {
			t.Fatalf("%s: fixture line %q has no \"trim\"", c.name, lines[c.line])
		}
		val, ok := hoverValue(src, c.line, col)
		if !ok {
			t.Fatalf("%s: hover result is null, want %q", c.name, want)
		}
		if val != want {
			t.Errorf("%s: hover value = %q, want %q", c.name, val, want)
		}
	}
}

// TestHoverDelegateMemberNameOnly covers a delegate member (no static sig):
// hover is name-only, no parens/arrow.
func TestHoverDelegateMemberNameOnly(t *testing.T) {
	src := "import \"array\"\n" +
		"\n" +
		"fn main() -> void {\n" +
		"  array.map(f, xs)\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	col := strings.Index(lines[3], "map")
	val, ok := hoverValue(src, 3, col)
	if !ok {
		t.Fatal("hover on array.map is null, want name-only detail")
	}
	if want := "(builtin) array.map"; val != want {
		t.Errorf("hover value = %q, want %q", val, want)
	}
}

// TestHoverQualifierModule covers T-LSP-3: hover on the namespace qualifier
// itself reports the module sense, not "(type) string".
func TestHoverQualifierModule(t *testing.T) {
	src := "import \"string\"\n" +
		"\n" +
		"fn main() -> void {\n" +
		"  string.trim(x)\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	col := strings.Index(lines[3], "string")
	val, ok := hoverValue(src, 3, col)
	if !ok {
		t.Fatal("hover on string qualifier is null, want (module) string")
	}
	if want := "(module) string"; val != want {
		t.Errorf("hover value = %q, want %q", val, want)
	}
}

// TestHoverBareTypeUnchanged is the T-LSP-3 regression guard: a bare type
// reference with no following dot is unaffected.
func TestHoverBareTypeUnchanged(t *testing.T) {
	src := "fn main() -> void {\n" +
		"  let s: string = \"hi\"\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	col := strings.Index(lines[1], "string")
	val, ok := hoverValue(src, 1, col)
	if !ok {
		t.Fatal("hover on bare type string is null")
	}
	if want := "(type) string"; val != want {
		t.Errorf("hover value = %q, want %q", val, want)
	}
}

// TestHoverFlatBuiltinUnchanged pins the unchanged flat-builtin hover path.
func TestHoverFlatBuiltinUnchanged(t *testing.T) {
	src := "fn main() -> void {\n" +
		"  length(\"hi\")\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	col := strings.Index(lines[1], "length")
	val, ok := hoverValue(src, 1, col)
	if !ok {
		t.Fatal("hover on length is null")
	}
	if want := "(builtin) length"; val != want {
		t.Errorf("hover value = %q, want %q", val, want)
	}
}

// TestHoverNamespacedMemberAllNamespaces pins criterion #1's "all 9
// namespaces" at the LSP layer (not just the types accessor), in call
// position, so a hover path that special-cases "string" cannot pass.
func TestHoverNamespacedMemberAllNamespaces(t *testing.T) {
	table := []struct{ ns, member string }{
		{"string", "trim"},
		{"array", "map"},
		{"dict", "has"},
		{"env", "get"},
		{"fs", "cwd"},
		{"json", "encode"},
		{"math", "floor"},
		{"process", "pid_alive"},
		{"regex", "matches"},
	}
	for _, tc := range table {
		lineText := "  " + tc.ns + "." + tc.member + "(x)"
		src := "fn main() -> void {\n" + lineText + "\n}\n"
		col := strings.Index(lineText, tc.member)
		val, ok := hoverValue(src, 1, col)
		if !ok {
			t.Fatalf("%s.%s: hover result is null", tc.ns, tc.member)
		}
		want := "(builtin) " + tc.ns + "."
		if !strings.HasPrefix(val, want) {
			t.Errorf("%s.%s: hover value = %q, want prefix %q", tc.ns, tc.member, val, want)
		}
	}
}

// TestHoverChainedFieldNotNamespace pins the bare-qualifier guard: a chained
// field access (obj.math.floor, where "math" is a struct field, not a
// namespace) must not false-match the core-module member or qualifier hover.
func TestHoverChainedFieldNotNamespace(t *testing.T) {
	src := "struct Sub { floor: int }\n" +
		"struct Obj { math: Sub }\n" +
		"fn main() -> void {\n" +
		"  let obj: Obj = Obj { math: Sub { floor: 1 } }\n" +
		"  obj.math.floor\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	line := lines[4]
	colFloor := strings.Index(line, "floor")
	colMath := strings.Index(line, "math")

	if val, ok := hoverValue(src, 4, colFloor); ok && val == "(builtin) math.floor" {
		t.Errorf("hover on floor in obj.math.floor = %q, want NOT (builtin) math.floor", val)
	}
	if val, ok := hoverValue(src, 4, colMath); ok && val == "(module) math" {
		t.Errorf("hover on math field in obj.math.floor = %q, want NOT (module) math", val)
	}
}

// TestDefinitionNamespacedMemberNull pins existing behavior: go-to-definition
// on a namespaced builtin member has no source and returns null, exactly like
// a flat builtin.
func TestDefinitionNamespacedMemberNull(t *testing.T) {
	src := "import \"string\"\n" +
		"\n" +
		"fn main() -> void {\n" +
		"  string.trim(x)\n" +
		"}\n"
	lines := strings.Split(src, "\n")
	col := strings.Index(lines[3], "trim")
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/definition", uri, 3, col))
	resp := respForID(msgs, 2)
	if resp == nil {
		t.Fatal("no definition response")
	}
	if v, present := resp["result"]; present && v != nil {
		t.Errorf("definition on string.trim = %v, want null", v)
	}
}

// completionLabels runs a completion request and returns the response's item
// labels as a set.
func completionLabels(src string, line, char int) map[string]bool {
	_, msgs := runServer(didOpenFrame(uri, src) + posReqFrame(2, "textDocument/completion", uri, line, char))
	resp := respForID(msgs, 2)
	labels := map[string]bool{}
	items, _ := resp["result"].([]any)
	for _, it := range items {
		labels[it.(map[string]any)["label"].(string)] = true
	}
	return labels
}

// TestCompletionCapabilityTrigger covers T-LSP-3's completion capability:
// the initialize result must advertise "." as a completion trigger character.
func TestCompletionCapabilityTrigger(t *testing.T) {
	_, msgs := runServer(reqFrame(1, "initialize", "{}"))
	resp := respForID(msgs, 1)
	result, _ := resp["result"].(map[string]any)
	caps, _ := result["capabilities"].(map[string]any)
	cp, _ := caps["completionProvider"].(map[string]any)
	if cp == nil {
		t.Fatalf("no completionProvider in capabilities: %v", caps)
	}
	triggers, _ := cp["triggerCharacters"].([]any)
	found := false
	for _, tc := range triggers {
		if tc == "." {
			found = true
		}
	}
	if !found {
		t.Errorf("completionProvider.triggerCharacters = %v, want it to contain \".\"", triggers)
	}
}

// TestCompletionNamespacePrefix covers T-LSP-3: completion after an "ns."
// prefix for a core namespace returns only that namespace's members, table-
// driven over string and array so no implementation can special-case one
// namespace and pass.
func TestCompletionNamespacePrefix(t *testing.T) {
	table := []struct {
		ns          string
		wantContain []string
	}{
		{"string", []string{"trim", "split", "join"}},
		{"array", []string{"map", "filter"}},
	}
	for _, tc := range table {
		lineText := "  " + tc.ns + "."
		src := "fn main() -> void {\n" + lineText + "\n}\n"
		labels := completionLabels(src, 1, len(lineText))
		for _, want := range tc.wantContain {
			if !labels[want] {
				t.Errorf("%s.: completion missing %q, got %v", tc.ns, want, labels)
			}
		}
		for _, absent := range []string{"if", "struct", "length"} {
			if labels[absent] {
				t.Errorf("%s.: completion must NOT contain %q, got %v", tc.ns, absent, labels)
			}
		}
	}
}

// TestCompletionUnqualifiedUnchanged pins the unchanged flat completion list
// mid-statement (not after an "ns." prefix).
func TestCompletionUnqualifiedUnchanged(t *testing.T) {
	src := "fn main() -> void {\n  \n}\n"
	labels := completionLabels(src, 1, 2)
	for _, want := range []string{"if", "length"} {
		if !labels[want] {
			t.Errorf("unqualified completion missing %q, got %v", want, labels)
		}
	}
}

// TestCompletionChainedFieldNoMembers pins the completion bare-qualifier
// guard: completion after a chained field access ("obj.math.") must NOT
// return math's members, since "math" here is a struct field, not a
// namespace -- it falls back to the flat list.
func TestCompletionChainedFieldNoMembers(t *testing.T) {
	src := "struct Sub { floor: int }\n" +
		"struct Obj { math: Sub }\n" +
		"fn main() -> void {\n" +
		"  let obj: Obj = Obj { math: Sub { floor: 1 } }\n" +
		"  obj.math.\n" +
		"}\n"
	lineText := "  obj.math."
	labels := completionLabels(src, 4, len(lineText))
	if labels["floor"] {
		t.Errorf("completion at obj.math. must NOT return math's member %q, got %v", "floor", labels)
	}
	if !labels["if"] {
		t.Errorf("completion at obj.math. should fall back to the flat list (missing %q), got %v", "if", labels)
	}
}
