package editors_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Asset paths, relative to this package directory (internal/editors).
const (
	vscodePkg     = "../../editors/vscode/package.json"
	vscodeGrammar = "../../editors/vscode/syntaxes/wisp.tmLanguage.json"
	vscodeLangCfg = "../../editors/vscode/language-configuration.json"
	vscodeExtJS   = "../../editors/vscode/extension.js"
	vimSyntax     = "../../editors/vim/syntax/wisp.vim"
	vimFtdetect   = "../../editors/vim/ftdetect/wisp.vim"
)

// partition computes the four disjoint tooling scopes from the compiler's
// authoritative accessors (tooling plan Conventions).
func partition() (control, typeNames, builtin, consts []string) {
	typeset := map[string]bool{}
	for _, t := range types.TypeNames() {
		typeset[t] = true
	}
	minus := func(xs []string) []string {
		var out []string
		for _, x := range xs {
			if !typeset[x] {
				out = append(out, x)
			}
		}
		return out
	}
	return minus(token.Keywords()), types.TypeNames(), minus(types.BuiltinNames()), types.ReservedConstants()
}

func toSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func assertWordsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	gs, ws := toSet(got), toSet(want)
	for w := range ws {
		if !gs[w] {
			t.Errorf("%s: missing %q (in compiler set, absent from asset)", label, w)
		}
	}
	for g := range gs {
		if !ws[g] {
			t.Errorf("%s: extra %q (in asset, not in compiler set)", label, g)
		}
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	return m
}

// tmWords extracts the alternation words from a TextMate match regex like
// `\b(?:break|case|...)\b`.
func tmWords(match string) []string {
	m := strings.ReplaceAll(match, `\b`, "")
	m = strings.ReplaceAll(m, "(?:", "")
	m = strings.ReplaceAll(m, ")", "")
	var out []string
	for _, p := range strings.Split(m, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// group1Alternation extracts the pipe-separated words inside the FIRST bare
// capture group "(...)" of a regex like `\b(array|dict|...)(\.)([a-z]...)\b`.
// Unlike tmWords (which unwraps non-capturing "(?:...)" groups for the flat
// word-list rules), the `namespaced` rule uses bare capture groups, so it needs
// its own small extractor rather than tmWords verbatim.
func group1Alternation(match string) []string {
	start := strings.Index(match, "(")
	if start == -1 {
		return nil
	}
	end := strings.Index(match[start:], ")")
	if end == -1 {
		return nil
	}
	inner := match[start+1 : start+end]
	var out []string
	for _, p := range strings.Split(inner, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// vscodeNamespacedRule locates the repository's "namespaced" rule object.
func vscodeNamespacedRule(t *testing.T, grammar map[string]any) map[string]any {
	t.Helper()
	repo, _ := grammar["repository"].(map[string]any)
	if repo == nil {
		t.Fatal("grammar has no repository")
	}
	rule, _ := repo["namespaced"].(map[string]any)
	if rule == nil {
		t.Fatal(`repository is missing the "namespaced" rule`)
	}
	return rule
}

// includeOrder extracts the ordered list of "#include" targets from a
// patterns array.
func includeOrder(patterns []any) []string {
	var out []string
	for _, p := range patterns {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if inc, ok := m["include"].(string); ok {
			out = append(out, inc)
		}
	}
	return out
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// TestVSCodeNamespaceReconciliation asserts (spec F1=A / drift-guard step 1)
// that the `namespaced` rule's group-1 namespace alternation is set-equal to
// the compiler's authoritative CoreNamespaces(), so a namespace added to
// coreCatalog cannot silently fall out of the VSCode grammar.
func TestVSCodeNamespaceReconciliation(t *testing.T) {
	grammar := readJSON(t, vscodeGrammar)
	rule := vscodeNamespacedRule(t, grammar)
	match, _ := rule["match"].(string)
	assertWordsEqual(t, "vscode namespaced group-1 alternation", group1Alternation(match), types.CoreNamespaces())
}

// TestVSCodeNamespacedIncludeOrdering asserts (drift-guard step 3 / spec
// acceptance) that "#namespaced" precedes "#types" in BOTH the top-level
// patterns array and the string-interpolation patterns array, since a
// same-position tie in TextMate resolves to the first-listed rule.
func TestVSCodeNamespacedIncludeOrdering(t *testing.T) {
	grammar := readJSON(t, vscodeGrammar)

	top, _ := grammar["patterns"].([]any)
	topOrder := includeOrder(top)
	ti, tt := indexOf(topOrder, "#namespaced"), indexOf(topOrder, "#types")
	if ti == -1 || tt == -1 || ti >= tt {
		t.Errorf("top-level patterns: #namespaced must precede #types, got order %v", topOrder)
	}

	interp := findNamedObject(grammar, "meta.interpolation.wisp")
	if interp == nil {
		t.Fatal("meta.interpolation.wisp rule not found")
	}
	interpPatterns, _ := interp["patterns"].([]any)
	interpOrder := includeOrder(interpPatterns)
	ii, it := indexOf(interpOrder, "#namespaced"), indexOf(interpOrder, "#types")
	if ii == -1 || it == -1 || ii >= it {
		t.Errorf("interpolation patterns: #namespaced must precede #types, got order %v", interpOrder)
	}
}

// isLowerPipeAlternation reports whether s is a bare lowercase pipe-separated
// alternation body (e.g. `array\|dict\|env`), the shape of a Vim `\%(...\)`
// namespace-list group's inner content.
func isLowerPipeAlternation(s string) bool {
	if !strings.Contains(s, `\|`) {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && r != '|' && r != '\\' {
			return false
		}
	}
	return true
}

// vimAlternation extracts the innermost `\%(a\|b\|...\)` namespace-list
// alternation from the `syn match <groupName>` line whose match text contains
// marker (used to disambiguate the two syn-match lines for group wispBuiltin:
// `\ze` picks the wispNamespace qualifier line, `\@<=` picks the
// namespaced-member line). `\%(...\)` groups can nest (the qualifier and
// member matches both wrap the namespace alternation in an outer group), so
// this walks the string tracking group starts and returns the first popped
// (innermost) group whose content is a bare lowercase alternation.
func vimAlternation(content, groupName, marker string) []string {
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || f[0] != "syn" || f[1] != "match" || f[2] != groupName {
			continue
		}
		joined := strings.Join(f[3:], " ")
		if !strings.Contains(joined, marker) {
			continue
		}
		var starts []int
		for i := 0; i < len(joined); {
			switch {
			case strings.HasPrefix(joined[i:], `\%(`):
				starts = append(starts, i+3)
				i += 3
			case strings.HasPrefix(joined[i:], `\)`) && len(starts) > 0:
				start := starts[len(starts)-1]
				starts = starts[:len(starts)-1]
				inner := joined[start:i]
				i += 2
				if isLowerPipeAlternation(inner) {
					var out []string
					for _, p := range strings.Split(inner, `\|`) {
						if p != "" {
							out = append(out, p)
						}
					}
					return out
				}
			default:
				i++
			}
		}
	}
	return nil
}

// TestVimNamespaceReconciliation asserts (drift-guard step 3) that BOTH the
// wispNamespace qualifier match and the wispBuiltin namespaced-member match
// carry a namespace alternation set-equal to CoreNamespaces(). The member
// rule holds its own independent alternation, so reconciling only the
// qualifier would let the member alternation silently drop a namespace.
func TestVimNamespaceReconciliation(t *testing.T) {
	b, err := os.ReadFile(vimSyntax)
	if err != nil {
		t.Fatalf("read %s: %v", vimSyntax, err)
	}
	content := string(b)

	nsWords := vimAlternation(content, "wispNamespace", `\ze`)
	assertWordsEqual(t, "vim wispNamespace alternation", nsWords, types.CoreNamespaces())

	memberWords := vimAlternation(content, "wispBuiltin", `\@<=`)
	assertWordsEqual(t, "vim wispBuiltin namespaced-member alternation", memberWords, types.CoreNamespaces())
}

// walkGrammar collects, from the parsed grammar, a name->match map for objects
// carrying both, and the set of every "name" scope appearing anywhere.
func walkGrammar(v any, scopeMatch map[string]string, allNames map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		name, hasName := t["name"].(string)
		if hasName {
			allNames[name] = true
			if m, ok := t["match"].(string); ok {
				scopeMatch[name] = m
			}
		}
		for _, vv := range t {
			walkGrammar(vv, scopeMatch, allNames)
		}
	case []any:
		for _, vv := range t {
			walkGrammar(vv, scopeMatch, allNames)
		}
	}
}

func TestVSCodeGrammarWordSetsMatchCompiler(t *testing.T) {
	grammar := readJSON(t, vscodeGrammar)
	scopeMatch := map[string]string{}
	allNames := map[string]bool{}
	walkGrammar(grammar, scopeMatch, allNames)

	control, typeNames, builtin, consts := partition()
	assertWordsEqual(t, "grammar keyword.control.wisp", tmWords(scopeMatch["keyword.control.wisp"]), control)
	assertWordsEqual(t, "grammar storage.type.wisp", tmWords(scopeMatch["storage.type.wisp"]), typeNames)
	assertWordsEqual(t, "grammar support.function.builtin.wisp", tmWords(scopeMatch["support.function.builtin.wisp"]), builtin)
	assertWordsEqual(t, "grammar constant.language.wisp", tmWords(scopeMatch["constant.language.wisp"]), consts)

	// error must be a TYPE scope, never a keyword or builtin.
	if !toSet(tmWords(scopeMatch["storage.type.wisp"]))["error"] {
		t.Error("error is not in the type scope of the grammar")
	}

	// Per-name bucket assertions for the Optional names (spec acceptance 8, risk
	// 3): each must be in its EXPECTED scope and ABSENT from the others, so a
	// future mis-bucketing edit is caught.
	typeScope := toSet(tmWords(scopeMatch["storage.type.wisp"]))
	constScope := toSet(tmWords(scopeMatch["constant.language.wisp"]))
	builtinScope := toSet(tmWords(scopeMatch["support.function.builtin.wisp"]))
	if !typeScope["Optional"] {
		t.Error("Optional must be in the type scope")
	}
	for _, c := range []string{"Some", "None"} {
		if !constScope[c] {
			t.Errorf("%s must be in the constant scope", c)
		}
		if typeScope[c] || builtinScope[c] {
			t.Errorf("%s must NOT be a type/builtin scope word", c)
		}
	}
	for _, b := range []string{"is_some", "is_none", "unwrap", "unwrap_or"} {
		if !builtinScope[b] {
			t.Errorf("%s must be in the builtin scope", b)
		}
		if typeScope[b] || constScope[b] {
			t.Errorf("%s must NOT be a type/constant scope word", b)
		}
	}
}

func TestVSCodeGrammarStructuralScopes(t *testing.T) {
	grammar := readJSON(t, vscodeGrammar)
	scopeMatch := map[string]string{}
	allNames := map[string]bool{}
	walkGrammar(grammar, scopeMatch, allNames)

	for _, sc := range []string{
		"comment.line.double-slash.wisp",
		"constant.numeric.integer.wisp",
		"constant.numeric.float.wisp",
		"string.quoted.single.wisp",
		"string.quoted.double.wisp",
		"meta.interpolation.wisp",
		"keyword.operator.wisp",
		"entity.name.function.wisp",
	} {
		if !allNames[sc] {
			t.Errorf("grammar is missing the structural scope %q", sc)
		}
	}

	// Attachment checks: presence alone would pass even if a scope were attached
	// to the wrong pattern. Pin the two most prone to misattachment.
	if !hasFunctionDeclCapture(grammar) {
		t.Error("entity.name.function.wisp is not attached to the `fn <name>` capture")
	}
	dq := findNamedObject(grammar, "string.quoted.double.wisp")
	if dq == nil || !subtreeNames(dq)["meta.interpolation.wisp"] {
		t.Error("meta.interpolation.wisp is not nested inside the double-quoted string rule")
	}

	// namespaced rule's capture mapping must be exactly group 1 ->
	// support.other.namespace.wisp, group 3 -> support.function.builtin.wisp
	// (group 2 is the literal dot), so a swapped or mis-scoped capture is
	// caught rather than passing a mere presence check.
	ns := vscodeNamespacedRule(t, grammar)
	caps, _ := ns["captures"].(map[string]any)
	if caps == nil {
		t.Fatal("namespaced rule has no captures")
	}
	// Exactly groups 1 and 3 are scoped (group 2, the literal dot, is unscoped);
	// an extra/unexpected capture group must fail rather than pass silently.
	if len(caps) != 2 {
		t.Errorf("namespaced captures = %v keys, want exactly {1,3}", caps)
	}
	c1, _ := caps["1"].(map[string]any)
	if c1 == nil || c1["name"] != "support.other.namespace.wisp" {
		t.Errorf("namespaced capture group 1 = %v, want name support.other.namespace.wisp", c1)
	}
	c3, _ := caps["3"].(map[string]any)
	if c3 == nil || c3["name"] != "support.function.builtin.wisp" {
		t.Errorf("namespaced capture group 3 = %v, want name support.function.builtin.wisp", c3)
	}
}

// findNamedObject returns the first object in the grammar whose "name" == name.
func findNamedObject(v any, name string) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if n, _ := t["name"].(string); n == name {
			return t
		}
		for _, vv := range t {
			if r := findNamedObject(vv, name); r != nil {
				return r
			}
		}
	case []any:
		for _, vv := range t {
			if r := findNamedObject(vv, name); r != nil {
				return r
			}
		}
	}
	return nil
}

// subtreeNames collects every "name" scope appearing within v's subtree.
func subtreeNames(v any) map[string]bool {
	out := map[string]bool{}
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if n, ok := t["name"].(string); ok {
				out[n] = true
			}
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(v)
	return out
}

// hasFunctionDeclCapture reports whether some `fn`-matching rule attaches
// entity.name.function.wisp to a capture group (the function name).
func hasFunctionDeclCapture(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		if m, ok := t["match"].(string); ok && strings.Contains(m, "(fn)") {
			if caps, ok := t["captures"].(map[string]any); ok {
				for _, cv := range caps {
					if cm, ok := cv.(map[string]any); ok && cm["name"] == "entity.name.function.wisp" {
						return true
					}
				}
			}
		}
		for _, vv := range t {
			if hasFunctionDeclCapture(vv) {
				return true
			}
		}
	case []any:
		for _, vv := range t {
			if hasFunctionDeclCapture(vv) {
				return true
			}
		}
	}
	return false
}

func TestVSCodePackageContributes(t *testing.T) {
	pkg := readJSON(t, vscodePkg)
	grammar := readJSON(t, vscodeGrammar)
	grammarScope, _ := grammar["scopeName"].(string)
	if grammarScope == "" {
		t.Fatal("grammar has no scopeName")
	}

	contributes, _ := pkg["contributes"].(map[string]any)
	if contributes == nil {
		t.Fatal("package.json has no contributes")
	}

	// languages
	langs, _ := contributes["languages"].([]any)
	if len(langs) == 0 {
		t.Fatal("contributes.languages is empty")
	}
	lang := langs[0].(map[string]any)
	if lang["id"] != "wisp" {
		t.Errorf("language id = %v, want wisp", lang["id"])
	}
	exts := lang["extensions"].([]any)
	if !containsAny(exts, ".wisp") {
		t.Errorf("language extensions = %v, want to include .wisp", exts)
	}
	aliases, _ := lang["aliases"].([]any)
	if len(aliases) == 0 {
		t.Error("language aliases is empty (spec 4.4(1)/4.1 require aliases)")
	}
	if lang["configuration"] != "./language-configuration.json" {
		t.Errorf("language configuration = %v, want ./language-configuration.json", lang["configuration"])
	}

	// grammars
	grammars, _ := contributes["grammars"].([]any)
	if len(grammars) == 0 {
		t.Fatal("contributes.grammars is empty")
	}
	g := grammars[0].(map[string]any)
	if g["language"] != "wisp" {
		t.Errorf("grammar language = %v, want wisp", g["language"])
	}
	if g["scopeName"] != grammarScope {
		t.Errorf("contributes.grammars scopeName = %v, want %q (the grammar's own)", g["scopeName"], grammarScope)
	}
	if g["path"] != "./syntaxes/wisp.tmLanguage.json" {
		t.Errorf("grammar path = %v, want ./syntaxes/wisp.tmLanguage.json", g["path"])
	}

	// language-configuration.json must be valid JSON.
	_ = readJSON(t, vscodeLangCfg)
}

// singleBoundaryWord returns the identifier in a Vim pattern of the exact form
// `"\<word\>"`, or the form `"\<word\>\.\@!"` (a word boundary followed by a
// negative lookahead for a literal dot, used to carve a word out of a keyword
// cluster when it also starts a namespace-qualifier match), or false for any
// other pattern (numbers, comments, the function-name match, etc.).
func singleBoundaryWord(pat string) (string, bool) {
	pat = strings.Trim(pat, `"`)
	pat = strings.TrimSuffix(pat, `\.\@!`)
	if !strings.HasPrefix(pat, `\<`) || !strings.HasSuffix(pat, `\>`) {
		return "", false
	}
	inner := pat[2 : len(pat)-2]
	if inner == "" {
		return "", false
	}
	for _, c := range inner {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return "", false
		}
	}
	return inner, true
}

// TestVSCodeExtensionWiring asserts the extension actually wires up the LSP
// client (not just highlighting): an entry point, language activation, the
// server-path/enable settings, and the language-client dependency.
func TestVSCodeExtensionWiring(t *testing.T) {
	pkg := readJSON(t, vscodePkg)

	main, _ := pkg["main"].(string)
	if main == "" {
		t.Error("package.json has no main entry point")
	}
	if _, err := os.Stat(vscodeExtJS); err != nil {
		t.Errorf("extension entry %s not found: %v", vscodeExtJS, err)
	}

	events, _ := pkg["activationEvents"].([]any)
	if !containsAny(events, "onLanguage:wisp") {
		t.Errorf("activationEvents = %v, want to include onLanguage:wisp", events)
	}

	deps, _ := pkg["dependencies"].(map[string]any)
	if deps == nil || deps["vscode-languageclient"] == nil {
		t.Errorf("dependencies = %v, want vscode-languageclient", deps)
	}

	contributes, _ := pkg["contributes"].(map[string]any)
	cfg, _ := contributes["configuration"].(map[string]any)
	props, _ := cfg["properties"].(map[string]any)
	for _, key := range []string{"wisp.lsp.path", "wisp.lsp.enable"} {
		if props[key] == nil {
			t.Errorf("contributes.configuration is missing %q", key)
		}
	}

	// The entry point must connect a language client to wisp-lsp over stdio.
	b, err := os.ReadFile(vscodeExtJS)
	if err != nil {
		t.Fatalf("read %s: %v", vscodeExtJS, err)
	}
	js := string(b)
	for _, want := range []string{"vscode-languageclient", "LanguageClient", "documentSelector", "lsp.path"} {
		if !strings.Contains(js, want) {
			t.Errorf("extension.js missing %q (LSP client wiring)", want)
		}
	}
}

func containsAny(xs []any, want string) bool {
	for _, x := range xs {
		if s, ok := x.(string); ok && s == want {
			return true
		}
	}
	return false
}

func TestVimSyntaxWordSetsMatchCompiler(t *testing.T) {
	b, err := os.ReadFile(vimSyntax)
	if err != nil {
		t.Fatalf("read %s: %v", vimSyntax, err)
	}
	content := string(b)

	// group name -> words. A group's set is the union of its `syn keyword`
	// cluster and any single-word `\<word\>` `syn match` lines (the latter cover
	// words like `contains` that are reserved :syn keyword arguments).
	groupToWords := map[string][]string{}
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "syn" && f[1] == "keyword" {
			groupToWords[f[2]] = append(groupToWords[f[2]], f[3:]...)
			continue
		}
		if len(f) >= 4 && f[0] == "syn" && f[1] == "match" {
			if w, ok := singleBoundaryWord(strings.Join(f[3:], " ")); ok {
				groupToWords[f[2]] = append(groupToWords[f[2]], w)
			}
		}
	}

	control, typeNames, builtin, consts := partition()
	assertWordsEqual(t, "vim wispKeyword", groupToWords["wispKeyword"], control)
	assertWordsEqual(t, "vim wispType", groupToWords["wispType"], typeNames)
	assertWordsEqual(t, "vim wispBuiltin", groupToWords["wispBuiltin"], builtin)
	assertWordsEqual(t, "vim wispConstant", groupToWords["wispConstant"], consts)
	if !toSet(groupToWords["wispType"])["error"] {
		t.Error("error is not in the Vim type group")
	}

	// Per-name bucket assertions for the Optional names (acceptance 8, both
	// grammars).
	vimType := toSet(groupToWords["wispType"])
	vimConst := toSet(groupToWords["wispConstant"])
	vimBuiltin := toSet(groupToWords["wispBuiltin"])
	if !vimType["Optional"] {
		t.Error("vim: Optional must be in wispType")
	}
	for _, c := range []string{"Some", "None"} {
		if !vimConst[c] {
			t.Errorf("vim: %s must be in wispConstant", c)
		}
		if vimType[c] || vimBuiltin[c] {
			t.Errorf("vim: %s must NOT be in wispType/wispBuiltin", c)
		}
	}
	for _, b := range []string{"is_some", "is_none", "unwrap", "unwrap_or"} {
		if !vimBuiltin[b] {
			t.Errorf("vim: %s must be in wispBuiltin", b)
		}
		if vimType[b] || vimConst[b] {
			t.Errorf("vim: %s must NOT be in wispType/wispConstant", b)
		}
	}

	// Guard against silent drift: a word that is a reserved :syn keyword argument
	// (e.g. `contains`) put inside a `syn keyword` cluster is silently consumed by
	// vim as a modifier -- it sources without error and is NOT highlighted, yet a
	// naive parse would record it as present. Such words must use `syn match`
	// instead. Flag any that slipped into a keyword cluster.
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "syn" && f[1] == "keyword" {
			for _, w := range f[3:] {
				if vimSynArgWords[w] {
					t.Errorf("vim: %q is a reserved :syn keyword argument and must be a `syn match`, not in the %q cluster", w, f[2])
				}
			}
		}
	}
}

// vimSynArgWords are the bare-word arguments :syn keyword/match/region parse, so
// any of them used as a literal keyword must be declared via `syn match`.
var vimSynArgWords = map[string]bool{
	"contained": true, "containedin": true, "contains": true, "oneline": true,
	"fold": true, "display": true, "extend": true, "concealends": true,
	"conceal": true, "cchar": true, "nextgroup": true, "skipwhite": true,
	"skipnl": true, "skipempty": true, "transparent": true, "keepend": true,
	"excludenl": true, "matchgroup": true, "start": true, "end": true, "skip": true,
}

func TestVimSyntaxStructure(t *testing.T) {
	b, err := os.ReadFile(vimSyntax)
	if err != nil {
		t.Fatalf("read %s: %v", vimSyntax, err)
	}
	content := string(b)
	for _, want := range []string{
		`b:current_syntax`,
		`syn keyword`,
		`syn match`,
		`syn region`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("vim syntax missing %q", want)
		}
	}
	// Every highlight group the file defines must be linked to a standard group,
	// or it renders unhighlighted. Check each one (not just a sample), so losing
	// a `hi def link` for constants/floats/operators/strings/interpolation fails.
	for _, grp := range []string{
		"wispKeyword", "wispType", "wispNamespace", "wispBuiltin", "wispConstant",
		"wispComment", "wispNumber", "wispFloat", "wispStringSingle",
		"wispStringDouble", "wispStringEscape", "wispInterp", "wispInterpDelim",
		"wispOperator", "wispFunction",
	} {
		if !strings.Contains(content, "hi def link "+grp+" ") {
			t.Errorf("vim syntax missing `hi def link %s`", grp)
		}
	}
	// wispNamespace must link specifically to Identifier (the approved scope), not
	// merely have some link — a mislink to Type/Function would otherwise pass.
	if !strings.Contains(content, "hi def link wispNamespace Identifier") {
		t.Error("vim syntax: wispNamespace must link to Identifier")
	}

	// wispType keyword cluster must not contain `string` (it overlaps the
	// namespace qualifier), and the `\<string\>\.\@!` carve-out match must
	// exist so bare `string` still highlights as a type.
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "syn" && f[1] == "keyword" && f[2] == "wispType" {
			for _, w := range f[3:] {
				if w == "string" {
					t.Error(`vim: wispType keyword cluster must NOT contain "string" (it overlaps the namespace qualifier)`)
				}
			}
		}
	}
	if !strings.Contains(content, `\<string\>\.\@!`) {
		t.Error(`vim: missing the "\<string\>\.\@!" wispType carve-out match`)
	}

	// The wispBuiltin namespaced-member match must exist: namespace
	// alternation, lookbehind (\@<=), lowercase member class. Invisible to the
	// word-set extractor (singleBoundaryWord skips it), so without this
	// structural check, deleting member highlighting would pass all other
	// tests.
	foundMember := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "syn match wispBuiltin") && strings.Contains(line, `\@<=`) &&
			strings.Contains(line, `\%(`) && strings.Contains(line, `[a-z][a-z0-9_]*`) {
			foundMember = true
		}
	}
	if !foundMember {
		t.Error("vim: missing the wispBuiltin namespaced-member match (namespace alternation + \\@<= + [a-z][a-z0-9_]*)")
	}

	// wispInterp's contains= list must include wispNamespace so namespaced
	// calls highlight inside ${...} interpolation.
	interpFound := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "syn region wispInterp") {
			if strings.Contains(line, "contains=") && strings.Contains(line, "wispNamespace") {
				interpFound = true
			}
		}
	}
	if !interpFound {
		t.Error("vim: wispInterp region's contains= list must include wispNamespace")
	}

	// If vim is available, sourcing the file must load without error.
	if vim, err := exec.LookPath("vim"); err == nil {
		abs, _ := filepath.Abs(vimSyntax)
		cmd := exec.Command(vim, "-Nu", "NONE", "-es", "-c", "so "+abs, "-c", "q")
		cmd.Stdin = nil
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("vim failed to source %s: %v\n%s", vimSyntax, err, out)
		}
	}
}

func TestVimFtdetect(t *testing.T) {
	b, err := os.ReadFile(vimFtdetect)
	if err != nil {
		t.Fatalf("read %s: %v", vimFtdetect, err)
	}
	content := string(b)
	if !strings.Contains(content, "*.wisp") || !strings.Contains(content, "filetype=wisp") {
		t.Errorf("ftdetect must set filetype=wisp for *.wisp, got:\n%s", content)
	}
}

// reconcileReserved performs a two-direction comparison between the classified
// (known) words and the canonical reserved list:
//   - missingFromReserved: classified words absent from reserved (and not in allow)
//   - unaccountedReserved: reserved entries absent from both classified and allow
func reconcileReserved(classified, reserved, allow []string) (missingFromReserved, unaccountedReserved []string) {
	reservedSet := toSet(reserved)
	allowSet := toSet(allow)
	classifiedSet := toSet(classified)

	for _, w := range classified {
		if !reservedSet[w] && !allowSet[w] {
			missingFromReserved = append(missingFromReserved, w)
		}
	}
	for _, w := range reserved {
		if !classifiedSet[w] && !allowSet[w] {
			unaccountedReserved = append(unaccountedReserved, w)
		}
	}
	sort.Strings(missingFromReserved)
	sort.Strings(unaccountedReserved)
	return
}

// TestDriftGuardReservedNames verifies that the editor partition (all four
// scopes) is a perfect subset of types.ReservedNames(), and that every entry
// in ReservedNames() is either classified or explicitly allowed (Result is
// a type name but not highlighted as a keyword/builtin/const in editors).
func TestDriftGuardReservedNames(t *testing.T) {
	control, typeNames, builtin, consts := partition()
	var classified []string
	classified = append(classified, control...)
	classified = append(classified, typeNames...)
	classified = append(classified, builtin...)
	classified = append(classified, consts...)

	missing, unaccounted := reconcileReserved(classified, types.ReservedNames(), []string{"Result"})
	if len(missing) != 0 {
		t.Errorf("classified words absent from ReservedNames(): %v", missing)
	}
	if len(unaccounted) != 0 {
		t.Errorf("ReservedNames() entries not in any editor scope (and not in allow): %v", unaccounted)
	}
}

// TestDriftGuardReservedNames_ExecutableNegative uses the same reconcileReserved
// helper to prove it actually detects drift in both directions.
func TestDriftGuardReservedNames_ExecutableNegative(t *testing.T) {
	cases := []struct {
		name            string
		classified      []string
		reserved        []string
		allow           []string
		wantMissing     []string
		wantUnaccounted []string
	}{
		{
			name:            "synthetic reserved name not classified",
			classified:      []string{"if", "else"},
			reserved:        []string{"if", "else", "zzz_reserved"},
			allow:           nil,
			wantMissing:     nil,
			wantUnaccounted: []string{"zzz_reserved"},
		},
		{
			name:            "classified word absent from reserved",
			classified:      []string{"if", "else", "orphaned_word"},
			reserved:        []string{"if", "else"},
			allow:           nil,
			wantMissing:     []string{"orphaned_word"},
			wantUnaccounted: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missing, unaccounted := reconcileReserved(tc.classified, tc.reserved, tc.allow)
			if !stringSlicesEqual(missing, tc.wantMissing) {
				t.Errorf("missingFromReserved = %v, want %v", missing, tc.wantMissing)
			}
			if !stringSlicesEqual(unaccounted, tc.wantUnaccounted) {
				t.Errorf("unaccountedReserved = %v, want %v", unaccounted, tc.wantUnaccounted)
			}
		})
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestPartitionIsActuallyDisjoint is a guard on the test's own premise: if the
// partition ever overlapped, the equality assertions above would be ambiguous.
func TestPartitionIsActuallyDisjoint(t *testing.T) {
	control, typeNames, builtin, consts := partition()
	scopes := map[string][]string{"control": control, "type": typeNames, "builtin": builtin, "const": consts}
	names := []string{"control", "type", "builtin", "const"}
	sort.Strings(names)
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := toSet(scopes[names[i]]), scopes[names[j]]
			for _, w := range b {
				if a[w] {
					t.Errorf("scopes %q and %q both contain %q", names[i], names[j], w)
				}
			}
		}
	}
}
