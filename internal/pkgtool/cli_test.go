package pkgtool

import (
	"bytes"
	"strings"
	"testing"
)

// A `wisp add` in a directory with no wisp.json must report the failure under
// the `wisp: add:` prefix, like every other add-command diagnostic -- not leak
// the bare `open .../wisp.json: no such file` from os.ReadFile.
func TestAddMissingManifestPrefixesError(t *testing.T) {
	root := t.TempDir() // no wisp.json
	var stderr bytes.Buffer
	code := Add(root, AddArgs{Repo: "o/r", Git: "https://x/o/r.git", HasGit: true}, &fakeFetcher{}, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit for missing wisp.json, got 0")
	}
	got := stderr.String()
	if !strings.HasPrefix(got, "wisp: add: ") {
		t.Errorf("error not prefixed with %q:\n%s", "wisp: add: ", got)
	}
}

// Same for `wisp remove`: a missing wisp.json must surface under `wisp: remove:`.
func TestRemoveMissingManifestPrefixesError(t *testing.T) {
	root := t.TempDir() // no wisp.json
	var stderr bytes.Buffer
	code := Remove(root, "o/r", &fakeFetcher{}, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit for missing wisp.json, got 0")
	}
	got := stderr.String()
	if !strings.HasPrefix(got, "wisp: remove: ") {
		t.Errorf("error not prefixed with %q:\n%s", "wisp: remove: ", got)
	}
}
