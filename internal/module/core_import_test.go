package module

import "testing"

// coreModule returns the loaded synthetic module for reserved namespace name, or
// nil.
func coreModule(l *Linked, name string) *Module {
	for _, m := range l.Modules {
		if m.Core == name {
			return m
		}
	}
	return nil
}

func TestCoreImportBindsSyntheticModule(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp": `import "json"` + "\n" + mainSrc,
	})
	l := mustLoad(t, dir, "main.wisp")

	cm := coreModule(l, "json")
	if cm == nil {
		t.Fatalf("no synthetic module with Core == \"json\"; modules = %+v", l.Modules)
	}
	if cm.Prog == nil {
		t.Errorf("synthetic module Prog is nil; want an empty program")
	}
	root := l.Modules[0]
	if got, ok := root.Namespaces["json"]; !ok || got != cm.ID {
		t.Errorf("root namespace \"json\" = (%d, %v), want (%d, true)", got, ok, cm.ID)
	}
}

func TestCoreImportAlias(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp": `import "json" as j` + "\n" + mainSrc,
	})
	l := mustLoad(t, dir, "main.wisp")
	cm := coreModule(l, "json")
	if cm == nil {
		t.Fatal("no synthetic json module")
	}
	root := l.Modules[0]
	if _, ok := root.Namespaces["json"]; ok {
		t.Errorf("did not expect default alias \"json\" when aliased as j")
	}
	if got, ok := root.Namespaces["j"]; !ok || got != cm.ID {
		t.Errorf("root namespace \"j\" = (%d, %v), want (%d, true)", got, ok, cm.ID)
	}
}

func TestCoreImportDedupAcrossModules(t *testing.T) {
	// Root imports json; an included lib also imports json. Both must resolve to
	// ONE shared synthetic module id.
	dir := proj(t, map[string]string{
		"main.wisp": `import "json"` + "\n" +
			`include "./lib.wisp" as lib` + "\n" + mainSrc,
		"lib.wisp": `import "json"` + "\n" +
			`export fn f() -> int { return 0 }` + "\n",
	})
	l := mustLoad(t, dir, "main.wisp")

	var coreCount int
	for _, m := range l.Modules {
		if m.Core == "json" {
			coreCount++
		}
	}
	if coreCount != 1 {
		t.Fatalf("expected exactly 1 synthetic json module, got %d", coreCount)
	}
	cm := coreModule(l, "json")
	root := l.Modules[0]
	var lib *Module
	for _, m := range l.Modules {
		if m.Namespaces["json"] == cm.ID && m != root && m.Core == "" {
			lib = m
		}
	}
	if lib == nil {
		t.Fatal("included lib did not bind the shared json module")
	}
	if root.Namespaces["json"] != lib.Namespaces["json"] {
		t.Errorf("root and lib resolve json to different ids: %d vs %d",
			root.Namespaces["json"], lib.Namespaces["json"])
	}
}

func TestCoreImportDuplicateCollision(t *testing.T) {
	dir := proj(t, map[string]string{
		"main.wisp": `import "json"` + "\n" +
			`import "json"` + "\n" + mainSrc,
	})
	_, diags := load(t, dir, "main.wisp")
	wantDiag(t, diags, "already bound")
}

func TestCoreImportRealImportStillWorks(t *testing.T) {
	dir := proj(t, map[string]string{
		"wisp.json":                         `{"name":"app","main":"main.wisp"}`,
		"main.wisp":                         `import "acme/util"` + "\n" + mainSrc,
		".wisp/modules/acme/util/wisp.json": `{"name":"util","main":"u.wisp"}`,
		".wisp/modules/acme/util/u.wisp":    `export fn f() -> int { return 1 }` + "\n",
	})
	l := mustLoad(t, dir, "main.wisp")
	if coreModule(l, "json") != nil {
		t.Error("did not import json, but a synthetic json module appeared")
	}
	if _, ok := l.Modules[0].Namespaces["util"]; !ok {
		t.Error("real import acme/util did not bind namespace util")
	}
}

// TestCoreNamespaceNamesAreValidExplicitAliases guards the spec's corrected claim
// (Batch A env module): a core-namespace name that equals a flat builtin (env) is
// still a VALID explicit `as` alias, because validIdent rejects only lexer
// keywords, not builtins. fs/process/math are likewise valid.
func TestCoreNamespaceNamesAreValidExplicitAliases(t *testing.T) {
	for _, name := range []string{"env", "fs", "process", "math"} {
		if !validIdent(name) {
			t.Errorf("validIdent(%q) = false, want true (core namespace must be a valid explicit alias)", name)
		}
	}
}
