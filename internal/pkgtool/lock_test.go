package pkgtool

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/version"
)

func TestLockCanonicalSerializationStable(t *testing.T) {
	// This test exercises marshalLock ONLY (never readLock), so terse git/commit values
	// are fine: marshalLock does not validate. (readLock-path fixtures use allowlisted
	// URLs + full SHAs; see TestLockRoundTrip.)
	// Same logical graph, different insertion order -> identical bytes. Module b/c is
	// given a deps slice in NON-sorted input order (["m/z","m/a"]) so the assertion below
	// proves the DEPS ARRAY itself is sorted, not merely that top-level module keys sort.
	// (b/c also depends on m/a and m/z so the edges are well-formed; m/a and m/z carry no
	// deps. This test exercises marshalLock ONLY, never readLock, so terse git/commit
	// values and the absence of a root edge to m/a,m/z are fine -- marshalLock does not
	// validate or check reachability.)
	a := &lockfile{Version: 1, Modules: map[string]lockEntry{
		"b/c": {Git: "g2", Ref: "r2", Commit: hex40("2"), TreeDigest: "sha256:bb", Deps: []string{"m/z", "m/a"}},
		"m/a": {Git: "g1", Ref: "r1", Commit: hex40("1"), TreeDigest: "sha256:aa", Deps: nil},
		"m/z": {Git: "g3", Ref: "r3", Commit: hex40("3"), TreeDigest: "sha256:cc", Deps: nil},
	}}
	b1, err := marshalLock(a)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := marshalLock(a)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("marshalLock not deterministic across calls")
	}
	// Deps array must be emitted sorted. The previous assertion (index of "m/a" before
	// "m/z" anywhere in the output) was VACUOUS: m/a and m/z are also top-level module
	// keys that sort regardless of whether the deps slice is sorted. Assert instead on
	// bytes UNIQUE to b/c's deps array: the exact sorted serialization of that slice.
	// json.MarshalIndent renders a multi-element string array one element per line, so the
	// canonical (gofmt-irrelevant, encoder-fixed) form of b/c's sorted deps is:
	wantDeps := "\"deps\": [\n        \"m/a\",\n        \"m/z\"\n      ]"
	if !bytes.Contains(b1, []byte(wantDeps)) {
		t.Errorf("b/c deps array not emitted sorted as %q; output:\n%s", wantDeps, b1)
	}
}

func TestLockRoundTrip(t *testing.T) {
	// readLock validates git URL + full-SHA commit + ref + key + treeDigest format, so the
	// round-trip fixture must use an allowlisted URL, a full 40-hex commit, AND a valid
	// "sha256:" + 64-hex treeDigest.
	dir := t.TempDir()
	lf := &lockfile{Version: 1, Modules: map[string]lockEntry{
		"a/a": {Git: "https://ex/a.git", Ref: "r1", Commit: hex40("1"), TreeDigest: "sha256:" + hex64("1")},
	}}
	p := filepath.Join(dir, "wisp.lock")
	if err := writeLock(p, lf); err != nil {
		t.Fatal(err)
	}
	got, err := readLock(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Modules["a/a"].Commit != hex40("1") {
		t.Errorf("round-trip lost commit")
	}
}

func TestReadLockRejectsAbbreviatedCommit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.lock"),
		`{"version":1,"modules":{"a/a":{"git":"https://x/a.git","ref":"r1","commit":"abc1234","treeDigest":"sha256:aa","deps":[]}}}`)
	if _, err := readLock(filepath.Join(dir, "wisp.lock")); err == nil {
		t.Fatal("expected a hard error on an abbreviated/garbage commit SHA")
	}
}

func TestReadLockRejectsDanglingDepEdge(t *testing.T) {
	// Use a VALID treeDigest so the rejection is attributable to the dangling edge, not the
	// digest-format check (readLock now also validates treeDigest).
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.lock"),
		`{"version":1,"modules":{"a/a":{"git":"https://x/a.git","ref":"r1","commit":"`+hex40("1")+`","treeDigest":"sha256:`+hex64("1")+`","deps":["b/b"]}}}`)
	if _, err := readLock(filepath.Join(dir, "wisp.lock")); err == nil {
		t.Fatal("expected a hard error on a dangling dep edge (b/b has no module entry)")
	}
}

func TestReadLockRejectsMalformedTreeDigest(t *testing.T) {
	// readLock must reject a treeDigest that is empty or not "sha256:" + 64 lowercase hex.
	// (The earlier fixtures' "sha256:aa" only ever fed marshalLock/round-trip paths that do
	// not validate the digest; readLock now enforces the format on load.)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.lock"),
		`{"version":1,"modules":{"a/a":{"git":"https://x/a.git","ref":"r1","commit":"`+hex40("1")+`","treeDigest":"sha256:nothex","deps":[]}}}`)
	if _, err := readLock(filepath.Join(dir, "wisp.lock")); err == nil {
		t.Fatal("expected a hard error on a malformed treeDigest (not sha256:64-hex)")
	}
}

func TestTreeDigestDeterministicAndOrderIndependent(t *testing.T) {
	dir1 := t.TempDir()
	writeFile(t, filepath.Join(dir1, "a.wisp"), "AAA")
	writeFile(t, filepath.Join(dir1, "sub", "b.wisp"), "BBB")
	d1, err := treeDigest(dir1)
	if err != nil {
		t.Fatal(err)
	}
	// A second tree with identical contents but created in reverse order hashes same.
	dir2 := t.TempDir()
	writeFile(t, filepath.Join(dir2, "sub", "b.wisp"), "BBB")
	writeFile(t, filepath.Join(dir2, "a.wisp"), "AAA")
	d2, err := treeDigest(dir2)
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Errorf("digest order-dependent: %q vs %q", d1, d2)
	}
	// Different content -> different digest.
	dir3 := t.TempDir()
	writeFile(t, filepath.Join(dir3, "a.wisp"), "AAA")
	writeFile(t, filepath.Join(dir3, "sub", "b.wisp"), "DIFFERENT")
	d3, _ := treeDigest(dir3)
	if d3 == d1 {
		t.Errorf("digest collision across distinct contents")
	}
}

func TestLockStampsCompilerVersion(t *testing.T) {
	mods := map[string]resolved{
		"a/a": {git: "https://ex/a.git", ref: "r1", commit: hex40("1"), treeDigest: "sha256:" + hex64("1")},
	}
	lf := buildLockfile(mods, sortedKeys(mods))
	if lf.Wisp != version.Number {
		t.Errorf("buildLockfile Wisp = %q, want %q (version.Number)", lf.Wisp, version.Number)
	}
}

func TestLockWispFieldRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Non-empty Wisp: marshals with a "wisp" key and round-trips through readLock.
	lf := &lockfile{Version: 1, Wisp: "9.9.9", Modules: map[string]lockEntry{
		"a/a": {Git: "https://ex/a.git", Ref: "r1", Commit: hex40("1"), TreeDigest: "sha256:" + hex64("1")},
	}}
	b, err := marshalLock(lf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"wisp": "9.9.9"`)) {
		t.Errorf("marshalLock output missing wisp key; got:\n%s", b)
	}
	p := filepath.Join(dir, "wisp.lock")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLock(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Wisp != "9.9.9" {
		t.Errorf("readLock Wisp = %q, want %q", got.Wisp, "9.9.9")
	}

	// Empty Wisp: omitempty drops the key, and round-trips to "".
	lfEmpty := &lockfile{Version: 1, Modules: map[string]lockEntry{
		"a/a": {Git: "https://ex/a.git", Ref: "r1", Commit: hex40("1"), TreeDigest: "sha256:" + hex64("1")},
	}}
	bEmpty, err := marshalLock(lfEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bEmpty, []byte(`"wisp"`)) {
		t.Errorf("marshalLock emitted a wisp key for an empty Wisp; got:\n%s", bEmpty)
	}
	pEmpty := filepath.Join(dir, "wisp-empty.lock")
	if err := os.WriteFile(pEmpty, bEmpty, 0o644); err != nil {
		t.Fatal(err)
	}
	gotEmpty, err := readLock(pEmpty)
	if err != nil {
		t.Fatal(err)
	}
	if gotEmpty.Wisp != "" {
		t.Errorf("readLock Wisp = %q, want empty", gotEmpty.Wisp)
	}
}

func TestWarnIfCompilerMismatch(t *testing.T) {
	cases := []struct {
		name           string
		recorded       string
		running        string
		wantSubstrings []string
		wantEmpty      bool
	}{
		{name: "mismatch", recorded: "1.0.0", running: "2.0.0", wantSubstrings: []string{"1.0.0", "2.0.0"}},
		{name: "match", recorded: "1.0.0", running: "1.0.0", wantEmpty: true},
		{name: "empty recorded", recorded: "", running: "1.0.0", wantEmpty: true},
		{name: "dev recorded", recorded: "0.0.0-dev", running: "1.0.0", wantEmpty: true},
		{name: "dev running", recorded: "1.0.0", running: "0.0.0-dev", wantEmpty: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			warnIfCompilerMismatch(&buf, c.recorded, c.running)
			out := buf.String()
			if c.wantEmpty {
				if out != "" {
					t.Errorf("warnIfCompilerMismatch(%q, %q) = %q, want empty", c.recorded, c.running, out)
				}
				return
			}
			if strings.Count(out, "wisp: warning:") != 1 {
				t.Errorf("warnIfCompilerMismatch(%q, %q) = %q, want exactly one warning line", c.recorded, c.running, out)
			}
			for _, s := range c.wantSubstrings {
				if !strings.Contains(out, s) {
					t.Errorf("warnIfCompilerMismatch(%q, %q) = %q, want it to contain %q", c.recorded, c.running, out, s)
				}
			}
		})
	}
}

func TestTreeDigestExcludesMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.wisp"), "AAA")
	d1, _ := treeDigest(dir)
	if err := os.Chmod(filepath.Join(dir, "a.wisp"), 0o755); err != nil {
		t.Skipf("chmod unsupported: %v", err)
	}
	d2, _ := treeDigest(dir)
	if d1 != d2 {
		t.Errorf("digest changed with mode; exec bit must be excluded")
	}
}
