package pkgtool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifestObjectDeps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"git":"https://x/owner/repo.git","ref":"v1"}}}`)
	m, err := readManifest(filepath.Join(dir, "wisp.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := m.Dependencies["owner/repo"]
	if !ok {
		t.Fatal("owner/repo missing")
	}
	if d.Git != "https://x/owner/repo.git" || d.Ref != "v1" {
		t.Errorf("dep = %+v", d)
	}
}

func TestReadManifestRejectsBareStringDep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":"git@..."}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on bare-string dependency value")
	}
}

func TestReadManifestRejectsMissingGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"ref":"v1"}}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on a dependency missing git")
	}
}

func TestReadManifestRejectsMissingRef(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"git":"https://x"}}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on a dependency missing ref")
	}
}

func TestReadManifestRejectsBadDepKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"../evil":{"git":"https://x","ref":"v1"}}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on invalid dependency key")
	}
}

func TestReadManifestRejectsDisallowedGitURL(t *testing.T) {
	// A disallowed transport (ext::) in a dependency git URL must be rejected at LOAD time
	// (via validateGitURL), not deep in install.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"git":"ext::sh -c evil","ref":"v1"}}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on a disallowed git transport in a dependency")
	}
}

func TestReadManifestRejectsBadRef(t *testing.T) {
	// A ref with a leading dash (git-flag injection) must be rejected at LOAD time via
	// validateRef.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"git":"https://x/o/r.git","ref":"-rf"}}}`)
	if _, err := readManifest(filepath.Join(dir, "wisp.json")); err == nil {
		t.Fatal("expected error on a malformed ref")
	}
}

func TestWriteManifestObjectForm(t *testing.T) {
	dir := t.TempDir()
	m := &manifest{
		Name: "root", Main: "main.wisp",
		Dependencies: map[string]dependency{"owner/repo": {Git: "https://x", Ref: "v1"}},
	}
	p := filepath.Join(dir, "wisp.json")
	if err := writeManifest(p, m); err != nil {
		t.Fatal(err)
	}
	got, err := readManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dependencies["owner/repo"].Git != "https://x" {
		t.Errorf("round-trip lost git: %+v", got)
	}
}

func TestWriteManifestPreservesUnknownFields(t *testing.T) {
	// A wisp.json with extra top-level fields (version, author, custom) must
	// survive a read/write cycle unchanged -- add/remove must not silently discard
	// fields the tool does not model (L7 fix).
	dir := t.TempDir()
	p := filepath.Join(dir, "wisp.json")
	writeFile(t, p, `{"name":"root","main":"main.wisp","version":"1.2.3","author":"alice","dependencies":{"owner/repo":{"git":"https://x/owner/repo.git","ref":"v1"}}}`)

	m, err := readManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an add by mutating dependencies, then write back.
	m.Dependencies["owner/pkg"] = dependency{Git: "https://x/owner/pkg.git", Ref: "v2"}
	if err := writeManifest(p, m); err != nil {
		t.Fatal(err)
	}

	// Re-read and verify extra fields survived.
	m2, err := readManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Name != "root" {
		t.Errorf("name lost after round-trip: got %q", m2.Name)
	}
	if _, ok := m2.Dependencies["owner/repo"]; !ok {
		t.Error("original dependency lost after round-trip")
	}
	if _, ok := m2.Dependencies["owner/pkg"]; !ok {
		t.Error("added dependency lost after round-trip")
	}
	// Check that the extra fields are present in the raw output.
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"version"`, `"1.2.3"`, `"author"`, `"alice"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("extra field %s not found in written wisp.json:\n%s", want, raw)
		}
	}
}
