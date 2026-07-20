package pkgtool

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFakeFetcherLaysFixture(t *testing.T) {
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees: map[string]map[string]string{
			hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "export fn x() -> int { return 0 }\n"},
		},
	}
	sha, err := f.Resolve("https://x/o/r.git", "v1")
	if err != nil || sha != hex40("1") {
		t.Fatalf("resolve = %q, %v", sha, err)
	}
	dest := t.TempDir()
	if err := f.Fetch("https://x/o/r.git", "v1", sha, dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "wisp.json")); err != nil || string(b) == "" {
		t.Fatalf("fixture not laid: %v", err)
	}
}

func TestFakeFetcherInjectedFailure(t *testing.T) {
	f := &fakeFetcher{
		resolve:   map[refKey]string{{"u", "v1"}: hex40("1")},
		trees:     map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
		failFetch: map[string]bool{hex40("1"): true},
	}
	if err := f.Fetch("u", "v1", hex40("1"), t.TempDir()); err == nil {
		t.Fatal("expected injected fetch failure")
	}
}
