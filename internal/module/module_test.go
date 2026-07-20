package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// proj writes a set of files (rel path -> content) under a fresh temp dir and
// returns the dir.
func proj(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// load resolves rootRel within dir.
func load(t *testing.T, dir, rootRel string) (*Linked, []Diag) {
	t.Helper()
	root := filepath.Join(dir, rootRel)
	b, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	return Load(root, string(b), OSFS{})
}

func mustLoad(t *testing.T, dir, rootRel string) *Linked {
	t.Helper()
	l, diags := load(t, dir, rootRel)
	if len(diags) > 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	return l
}

func wantDiag(t *testing.T, diags []Diag, substr string) {
	t.Helper()
	for _, d := range diags {
		if strings.Contains(d.Msg, substr) {
			return
		}
	}
	t.Fatalf("no diag contains %q; got %v", substr, diags)
}

const mainSrc = "fn main() -> int { return 0 }\n"

func TestProjectRootDiscovery(t *testing.T) {
	// Nested wisp.json: the nearest ancestor is the root.
	dir := proj(t, map[string]string{
		"wisp.json":     `{"name":"root","main":"main.wisp"}`,
		"sub/app.wisp":  mainSrc,
		"sub/wisp.json": `{"name":"sub","main":"app.wisp"}`,
	})
	l := mustLoad(t, dir, "sub/app.wisp")
	wantRoot, _ := filepath.EvalSymlinks(filepath.Join(dir, "sub"))
	if l.Root != wantRoot {
		t.Errorf("nearest-ancestor root = %q, want %q", l.Root, wantRoot)
	}

	// No wisp.json anywhere: the source file's own dir is the root.
	dir2 := proj(t, map[string]string{"a/main.wisp": mainSrc})
	l2 := mustLoad(t, dir2, "a/main.wisp")
	wantRoot2, _ := filepath.EvalSymlinks(filepath.Join(dir2, "a"))
	if l2.Root != wantRoot2 {
		t.Errorf("no-manifest root = %q, want %q", l2.Root, wantRoot2)
	}
}

func TestIncludeHappyPath(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp":     `include "./lib/util.wisp"` + "\n" + `include "./other.wisp" as o` + "\n" + mainSrc,
		"lib/util.wisp": "export fn helper() -> int { return 1 }\n",
		"other.wisp":    "export fn f() -> int { return 2 }\n",
	})
	l := mustLoad(t, dir, "main.wisp")
	if len(l.Modules) != 3 {
		t.Fatalf("want 3 modules, got %d", len(l.Modules))
	}
	ns := l.Modules[0].Namespaces
	if ns["util"] == 0 || ns["o"] == 0 {
		t.Errorf("namespaces = %v, want util and o bound to non-root ids", ns)
	}
}

func TestIncludeRejections(t *testing.T) {
	cases := []struct {
		name, root string
		files      map[string]string
		want       string
	}{
		{"missing", "main.wisp", map[string]string{"main.wisp": `include "./nope.wisp"` + "\n" + mainSrc}, "cannot find"},
		{"non-wisp", "main.wisp", map[string]string{"main.wisp": `include "./x.txt"` + "\n" + mainSrc, "x.txt": "x"}, "must end in .wisp"},
		{"absolute", "main.wisp", map[string]string{"main.wisp": `include "/etc/passwd.wisp"` + "\n" + mainSrc}, "must be relative"},
		{"escape", "sub/main.wisp", map[string]string{"sub/main.wisp": `include "../secret.wisp"` + "\n" + mainSrc, "secret.wisp": "export fn s() -> int { return 0 }\n"}, "escapes its containment root"},
		{"keyword-stem", "main.wisp", map[string]string{"main.wisp": `include "./if.wisp"` + "\n" + mainSrc, "if.wisp": "export fn x() -> int { return 0 }\n"}, "not a valid identifier"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := proj(t, c.files)
			_, diags := load(t, dir, c.root)
			wantDiag(t, diags, c.want)
		})
	}
}

func TestIncludeEscapeViaSymlink(t *testing.T) {
	dir := proj(t, map[string]string{
		"proj/main.wisp": `include "./link.wisp"` + "\n" + mainSrc,
		"outside.wisp":   "export fn o() -> int { return 0 }\n",
	})
	// Symlink proj/link.wisp -> ../outside.wisp (escapes the project root).
	if err := os.Symlink(filepath.Join(dir, "outside.wisp"), filepath.Join(dir, "proj", "link.wisp")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, diags := load(t, dir, "proj/main.wisp")
	wantDiag(t, diags, "escapes its containment root")
}

func TestRootIncludeIntoWispRejected(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp":                   `include "./.wisp/modules/o/r/x.wisp"` + "\n" + mainSrc,
		".wisp/modules/o/r/x.wisp":    "export fn x() -> int { return 0 }\n",
		".wisp/modules/o/r/wisp.json": `{"name":"r","main":"x.wisp"}`,
	})
	_, diags := load(t, dir, "main.wisp")
	wantDiag(t, diags, "cannot reach into .wisp/")
}

func TestInPackageSelfIncludeAllowed(t *testing.T) {
	dir := proj(t, map[string]string{
		"wisp.json":                       `{"name":"root","main":"main.wisp"}`,
		"main.wisp":                       `import "o/r"` + "\n" + mainSrc,
		".wisp/modules/o/r/wisp.json":     `{"name":"r","main":"entry.wisp"}`,
		".wisp/modules/o/r/entry.wisp":    `include "./internal.wisp"` + "\n" + "export fn pub() -> int { return internal.priv() }\n",
		".wisp/modules/o/r/internal.wisp": "export fn priv() -> int { return 7 }\n",
	})
	l := mustLoad(t, dir, "main.wisp")
	// root + package entry + package internal = 3 modules.
	if len(l.Modules) != 3 {
		t.Fatalf("want 3 modules, got %d: %v", len(l.Modules), modPaths(l))
	}
}

func TestImportHappyPath(t *testing.T) {
	dir := proj(t, map[string]string{
		"wisp.json":                          `{"name":"root","main":"main.wisp"}`,
		"main.wisp":                          `import "owner/repo"` + "\n" + `import "a/b" as bee` + "\n" + mainSrc,
		".wisp/modules/owner/repo/wisp.json": `{"name":"repo","main":"lib.wisp"}`,
		".wisp/modules/owner/repo/lib.wisp":  "export fn get() -> int { return 1 }\n",
		".wisp/modules/a/b/wisp.json":        `{"name":"bpkg","main":"m.wisp"}`,
		".wisp/modules/a/b/m.wisp":           "export fn f() -> int { return 2 }\n",
	})
	l := mustLoad(t, dir, "main.wisp")
	ns := l.Modules[0].Namespaces
	if _, ok := ns["repo"]; !ok { // default alias = package name
		t.Errorf("default import alias `repo` not bound: %v", ns)
	}
	if _, ok := ns["bee"]; !ok { // explicit alias
		t.Errorf("explicit import alias `bee` not bound: %v", ns)
	}
}

func TestTypeKeywordAliasAccepted(t *testing.T) {
	// Type-keyword tokens (string, int, bool, float, error) are accepted as
	// import aliases, include aliases, and package `Name` values -- they are
	// value-level builtin names, not reserved at the identifier-binding layer.
	t.Run("import-alias", func(t *testing.T) {
		dir := proj(t, map[string]string{
			"wisp.json":                          `{"name":"root","main":"main.wisp"}`,
			"main.wisp":                          `import "owner/repo" as string` + "\n" + mainSrc,
			".wisp/modules/owner/repo/wisp.json": `{"name":"repo","main":"lib.wisp"}`,
			".wisp/modules/owner/repo/lib.wisp":  "export fn get() -> int { return 1 }\n",
		})
		l := mustLoad(t, dir, "main.wisp")
		if _, ok := l.Modules[0].Namespaces["string"]; !ok {
			t.Errorf("import alias `string` not bound: %v", l.Modules[0].Namespaces)
		}
	})
	t.Run("include-alias", func(t *testing.T) {
		dir := proj(t, map[string]string{
			"main.wisp":  `include "./other.wisp" as int` + "\n" + mainSrc,
			"other.wisp": "export fn f() -> int { return 2 }\n",
		})
		l := mustLoad(t, dir, "main.wisp")
		if l.Modules[0].Namespaces["int"] == 0 {
			t.Errorf("include alias `int` not bound: %v", l.Modules[0].Namespaces)
		}
	})
	t.Run("package-name", func(t *testing.T) {
		dir := proj(t, map[string]string{
			"wisp.json":                   `{"name":"root","main":"main.wisp"}`,
			"main.wisp":                   `import "o/r"` + "\n" + mainSrc,
			".wisp/modules/o/r/wisp.json": `{"name":"bool","main":"m.wisp"}`,
			".wisp/modules/o/r/m.wisp":    "export fn x() -> int { return 0 }\n",
		})
		l := mustLoad(t, dir, "main.wisp")
		if _, ok := l.Modules[0].Namespaces["bool"]; !ok {
			t.Errorf("default alias `bool` (from package Name) not bound: %v", l.Modules[0].Namespaces)
		}
	})
}

func TestImportRejections(t *testing.T) {
	base := func(extra map[string]string, mainBody string) map[string]string {
		m := map[string]string{
			"wisp.json": `{"name":"root","main":"main.wisp"}`,
			"main.wisp": mainBody + "\n" + mainSrc,
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	cases := []struct {
		name, body string
		extra      map[string]string
		want       string
	}{
		{"missing-pkg", `import "o/r"`, nil, "package not found"},
		{"malformed-json", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": "{bad", ".wisp/modules/o/r/m.wisp": "x"}, "malformed wisp.json"},
		{"missing-name", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"main":"m.wisp"}`, ".wisp/modules/o/r/m.wisp": "x"}, `no "name"`},
		{"invalid-name", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"bad name","main":"m.wisp"}`, ".wisp/modules/o/r/m.wisp": "x"}, "not a valid identifier"},
		{"metachar-name", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"a$(rm)","main":"m.wisp"}`, ".wisp/modules/o/r/m.wisp": "x"}, "not a valid identifier"},
		{"missing-entry", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"r","main":"gone.wisp"}`}, "entry file"},
		{"abs-entry", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"r","main":"/etc/x.wisp"}`}, "relative .wisp path"},
		{"nonwisp-entry", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"r","main":"x.txt"}`, ".wisp/modules/o/r/x.txt": "x"}, "relative .wisp path"},
		{"malformed-repo-1", `import "a"`, nil, "exactly owner/repo"},
		{"malformed-repo-3", `import "a/b/c"`, nil, "exactly owner/repo"},
		{"malformed-repo-dotdot", `import "../b"`, nil, "invalid segment"},
		{"malformed-repo-dot", `import "owner/."`, nil, "invalid segment"},
		{"keyword-alias", `import "o/r" as if`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"r","main":"m.wisp"}`, ".wisp/modules/o/r/m.wisp": "export fn x() -> int { return 0 }\n"}, "expected Ident"},
		{"keyword-name", `import "o/r"`, map[string]string{".wisp/modules/o/r/wisp.json": `{"name":"return","main":"m.wisp"}`, ".wisp/modules/o/r/m.wisp": "export fn x() -> int { return 0 }\n"}, "not a valid identifier"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := proj(t, base(c.extra, c.body))
			_, diags := load(t, dir, "main.wisp")
			wantDiag(t, diags, c.want)
		})
	}
}

func TestMissingImportNamesRootAndPath(t *testing.T) {
	dir := proj(t, map[string]string{
		"wisp.json": `{"name":"root","main":"main.wisp"}`,
		"main.wisp": `import "owner/repo"` + "\n" + mainSrc,
	})
	_, diags := load(t, dir, "main.wisp")
	if len(diags) == 0 {
		t.Fatal("expected a diag")
	}
	msg := diags[0].Msg
	root, _ := filepath.EvalSymlinks(dir)
	if !strings.Contains(msg, root) {
		t.Errorf("error %q does not name the resolved root %q", msg, root)
	}
	if !strings.Contains(msg, filepath.Join("modules", "owner", "repo")) {
		t.Errorf("error %q does not name the looked-under path", msg)
	}
}

func TestDedupDiamondAndAliases(t *testing.T) {
	// Diamond: main includes a and b, both include shared. Two aliases for shared
	// in main too. One Module for shared.
	dir := proj(t, map[string]string{
		"main.wisp":   `include "./a.wisp"` + "\n" + `include "./b.wisp"` + "\n" + `include "./shared.wisp" as s1` + "\n" + `include "./shared.wisp" as s2` + "\n" + mainSrc,
		"a.wisp":      `include "./shared.wisp"` + "\n" + "export fn fa() -> int { return shared.v() }\n",
		"b.wisp":      `include "./shared.wisp"` + "\n" + "export fn fb() -> int { return shared.v() }\n",
		"shared.wisp": "export fn v() -> int { return 9 }\n",
	})
	l := mustLoad(t, dir, "main.wisp")
	// root, a, b, shared = 4 modules (shared compiled once).
	if len(l.Modules) != 4 {
		t.Fatalf("want 4 modules (shared deduped), got %d: %v", len(l.Modules), modPaths(l))
	}
	ns := l.Modules[0].Namespaces
	if ns["s1"] != ns["s2"] {
		t.Errorf("two aliases for shared resolve to different modids: %v", ns)
	}
}

func TestDedupViaSymlink(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp":   `include "./shared.wisp"` + "\n" + `include "./alias.wisp" as a` + "\n" + mainSrc,
		"shared.wisp": "export fn v() -> int { return 1 }\n",
	})
	if err := os.Symlink(filepath.Join(dir, "shared.wisp"), filepath.Join(dir, "alias.wisp")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	l := mustLoad(t, dir, "main.wisp")
	if len(l.Modules) != 2 { // root + shared (alias.wisp is the same file)
		t.Fatalf("want 2 modules (symlink deduped), got %d: %v", len(l.Modules), modPaths(l))
	}
}

func TestCycle(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp": `include "./a.wisp"` + "\n" + mainSrc,
		"a.wisp":    `include "./b.wisp"` + "\n" + "export fn fa() -> int { return b.fb() }\n",
		"b.wisp":    `include "./a.wisp"` + "\n" + "export fn fb() -> int { return a.fa() }\n",
	})
	_, diags := load(t, dir, "main.wisp")
	wantDiag(t, diags, "cycle")
}

func TestBindingCollision(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp":     `include "./util.wisp"` + "\n" + `include "./lib/util.wisp"` + "\n" + mainSrc,
		"util.wisp":     "export fn a() -> int { return 1 }\n",
		"lib/util.wisp": "export fn b() -> int { return 2 }\n",
	})
	_, diags := load(t, dir, "main.wisp")
	wantDiag(t, diags, "already bound")
}

func TestExtraManifestFieldsAndAbsentDep(t *testing.T) {
	// A manifest with extra (M9) fields still builds; a declared-but-absent
	// dependency only errors when actually imported.
	dir := proj(t, map[string]string{
		"wisp.json": `{"name":"root","main":"main.wisp","dependencies":{"x/y":"git@..."}}`,
		"main.wisp": mainSrc, // does not import x/y
	})
	mustLoad(t, dir, "main.wisp") // builds despite the declared-but-absent dep

	dir2 := proj(t, map[string]string{
		"wisp.json": `{"name":"root","main":"main.wisp","dependencies":{"x/y":"git@..."}}`,
		"main.wisp": `import "x/y"` + "\n" + mainSrc,
	})
	_, diags := load(t, dir2, "main.wisp")
	wantDiag(t, diags, "package not found")
}

func modPaths(l *Linked) []string {
	var out []string
	for _, m := range l.Modules {
		out = append(out, filepath.Base(m.Path))
	}
	return out
}
