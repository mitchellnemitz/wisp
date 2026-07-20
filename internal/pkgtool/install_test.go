package pkgtool

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/version"
)

// project lays a root project with the given wisp.json content and returns its dir.
func project(t *testing.T, wispJSON string) string {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wisp.json"), wispJSON)
	writeFile(t, filepath.Join(dir, "main.wisp"), "fn main() -> int { return 0 }\n")
	return dir
}

func TestInstallFromScratchWritesLockAndModules(t *testing.T) {
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees: map[string]map[string]string{hex40("1"): {
			"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "export fn x() -> int { return 0 }\n"}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r", "wisp.json")); err != nil {
		t.Fatalf("module not placed: %v", err)
	}
	lf, err := readLock(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	e := lf.Modules["o/r"]
	if e.Commit != hex40("1") || e.Ref != "v1" || e.TreeDigest == "" {
		t.Errorf("lock entry incomplete: %+v", e)
	}
}

func TestInstallConsistentFailureOnCleanCheckoutLeavesNoWispDir(t *testing.T) {
	// A consistent lock exists (committed) but .wisp/modules is absent (clean checkout).
	// The fetcher fails; the failed install must remove the .wisp/ it created, leaving
	// the project byte-identical (atomicity), including on the consistent-path early
	// returns (not only apply-phase failures).
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	// A consistent lock referencing o/r, but no .wisp/modules on disk.
	lock := `{"version":1,"modules":{"o/r":{"git":"https://x/o/r.git","ref":"v1","commit":"` + hex40("1") + `","treeDigest":"sha256:` + hex64("a") + `","deps":[]}}}`
	writeFile(t, filepath.Join(dir, "wisp.lock"), lock)
	// Fetcher with no tree for the commit -> Fetch fails.
	f := &fakeFetcher{trees: map[string]map[string]string{}}
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected install to fail (fetcher has no tree)")
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp")); !os.IsNotExist(err) {
		t.Errorf(".wisp/ was left behind after a failed consistent install on a clean checkout (err=%v)", err)
	}
}

func TestInstallConsistentPrunesUnreachableDir(t *testing.T) {
	// A consistent install must prune an on-disk module dir not present in the lock,
	// so .wisp/modules is reproduced byte-identically (AC4).
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Plant a stray module dir not referenced by the lock.
	stray := filepath.Join(dir, ".wisp", "modules", "ghost", "pkg")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(stray, "wisp.json"), `{"name":"pkg","main":"m.wisp"}`)
	// A second (consistent) install should remove the stray dir.
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "ghost")); !os.IsNotExist(err) {
		t.Errorf("unreachable dir was not pruned on a consistent install (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r", "wisp.json")); err != nil {
		t.Errorf("reachable module wrongly removed: %v", err)
	}
}

func TestInstallConsistentLockNoFetch(t *testing.T) {
	// After one install, a counting fetcher must not be called on a second install
	// when the on-disk tree matches the locked digest.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	fetchesBefore := f.fetches
	resolvesBefore := f.resolves
	defaultRefsBefore := f.defaultRefs
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Pin-wins consistency (spec 4.3) forbids ANY remote query on a consistent-lock
	// install: no Fetch, AND no Resolve, AND no DefaultRef. Asserting only zero Fetch
	// would miss an install that still re-resolved refs against the remote.
	if f.fetches != fetchesBefore {
		t.Errorf("second install fetched %d times; want 0 new fetches", f.fetches-fetchesBefore)
	}
	if f.resolves != resolvesBefore {
		t.Errorf("second install made %d Resolve calls; want 0 (pin-wins forbids any remote query)", f.resolves-resolvesBefore)
	}
	if f.defaultRefs != defaultRefsBefore {
		t.Errorf("second install made %d DefaultRef calls; want 0 (pin-wins forbids any remote query)", f.defaultRefs-defaultRefsBefore)
	}
	// AC4: a consistent install leaves wisp.lock BYTE-identical.
	lockAfter, err := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lockBefore, lockAfter) {
		t.Errorf("consistent install rewrote wisp.lock; bytes must be identical")
	}
}

func TestInstallConsistentReusedTreeMissingRefetches(t *testing.T) {
	// AC4/reuse: on a consistent lock, a reachable module whose on-disk tree is MISSING
	// is re-fetched (and re-placed), not left absent.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Delete the on-disk module tree but keep the (consistent) lock.
	if err := os.RemoveAll(filepath.Join(dir, ".wisp", "modules", "o", "r")); err != nil {
		t.Fatal(err)
	}
	before := f.fetches
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	if f.fetches-before != 1 {
		t.Errorf("missing reused tree did %d fetches; want exactly 1 re-fetch", f.fetches-before)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r", "wisp.json")); err != nil {
		t.Errorf("missing tree was not reproduced: %v", err)
	}
}

func TestInstallConsistentCleanCheckoutRebuildsReachableSetFromLockEdges(t *testing.T) {
	// AC5 (clean checkout): a CONSISTENT lock (root deps unchanged) + an EMPTY
	// .wisp/modules/ (fresh clone) must reproduce the WHOLE reachable set -- including a
	// TRANSITIVE dep -- from the lock's `deps` edges ALONE. The graph is
	// root -> o/parent ; o/parent -> o/child (transitive). After a first install, wipe the
	// entire .wisp/modules/ tree (clean checkout) but keep the consistent lock. The second
	// install must NOT re-resolve the graph (pin-wins, no Resolve/DefaultRef) and must
	// materialize BOTH o/parent AND the transitive o/child from their pinned commits, with
	// the transitive child reached purely via the lock edge o/parent -> o/child. This is
	// DISTINCT from TestInstallConsistentReusedTreeMissingRefetches (which has no transitive
	// dep): here the reachable set is rebuilt by walking the lock's stored edges, proving the
	// transitive module is materialized without ever re-reading any manifest.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/p.git", "rp"}: hex40("a"),
			{"https://x/c.git", "rc"}: hex40("b"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"parent","main":"m.wisp","dependencies":{"o/child":{"git":"https://x/c.git","ref":"rc"}}}`, "m.wisp": "P"},
			hex40("b"): {"wisp.json": `{"name":"child","main":"m.wisp"}`, "m.wisp": "C"},
		},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/parent":{"git":"https://x/p.git","ref":"rp"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	// Clean checkout: wipe the ENTIRE module cache, keep the (consistent) lock + manifest.
	if err := os.RemoveAll(filepath.Join(dir, ".wisp", "modules")); err != nil {
		t.Fatal(err)
	}
	fetchesBefore := f.fetches
	resolvesBefore := f.resolves
	defaultRefsBefore := f.defaultRefs
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Pin-wins, so NO remote graph re-resolution: zero Resolve and zero DefaultRef. The two
	// absent reachable trees (o/parent + transitive o/child) are each materialized once.
	if f.resolves != resolvesBefore {
		t.Errorf("clean-checkout consistent install made %d Resolve calls; want 0 (rebuilt from lock edges)", f.resolves-resolvesBefore)
	}
	if f.defaultRefs != defaultRefsBefore {
		t.Errorf("clean-checkout consistent install made %d DefaultRef calls; want 0", f.defaultRefs-defaultRefsBefore)
	}
	if f.fetches-fetchesBefore != 2 {
		t.Errorf("clean-checkout install did %d fetches; want 2 (parent + transitive child, both absent)", f.fetches-fetchesBefore)
	}
	// Both the direct parent AND the transitive child must be materialized from the lock.
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "parent", "wisp.json")); err != nil {
		t.Errorf("direct o/parent not materialized on clean checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "child", "wisp.json")); err != nil {
		t.Errorf("transitive o/child not materialized from the lock edge on clean checkout: %v", err)
	}
	// Consistent path leaves the lock byte-identical.
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.lock")); !bytes.Equal(b, lockBefore) {
		t.Errorf("clean-checkout consistent install rewrote wisp.lock; bytes must be identical")
	}
}

func TestInstallConsistentTamperedTreeHardError(t *testing.T) {
	// AC4/reuse: on a consistent lock, a PRESENT on-disk tree whose recomputed digest
	// disagrees with the lock is a hard error (tampering/partial write), not a silent
	// re-fetch.
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Tamper with the on-disk tree so its digest no longer matches the lock.
	writeFile(t, filepath.Join(dir, ".wisp", "modules", "o", "r", "m.wisp"), "TAMPERED")
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected hard error on a tampered on-disk tree under a consistent lock")
	}
}

func TestInstallDigestMismatchHardError(t *testing.T) {
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Corrupt the locked digest so the on-disk tree mismatches the lock.
	lf, _ := readLock(filepath.Join(dir, "wisp.lock"))
	e := lf.Modules["o/r"]
	e.TreeDigest = "sha256:" + hex64("f")
	lf.Modules["o/r"] = e
	writeLock(filepath.Join(dir, "wisp.lock"), lf)
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected digest-mismatch hard error")
	}
}

func TestInstallMidExtractFailureLeavesNothing(t *testing.T) {
	f := &fakeFetcher{
		resolve:   map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:     map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
		failFetch: map[string]bool{hex40("1"): true},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r")); !os.IsNotExist(err) {
		t.Errorf("partial module tree left behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wisp.lock")); !os.IsNotExist(err) {
		t.Errorf("partial lock written on failure: %v", err)
	}
}

func TestInstallStaleReusesUnchangedModules(t *testing.T) {
	// root deps: o/a (changes) + o/keep (unchanged). After bumping o/a's ref, o/keep
	// must not be re-fetched, o/a must be.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "r1"}: hex40("a"), {"https://x/a.git", "r2"}: hex40("b"),
			{"https://x/k.git", "rk"}: hex40("c"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`},
			hex40("b"): {"wisp.json": `{"name":"a","main":"m.wisp"}`},
			hex40("c"): {"wisp.json": `{"name":"keep","main":"m.wisp"}`},
		},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r1"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Bump o/a's ref to r2.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r2"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	before := f.fetches
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Exactly one new fetch (o/a@r2); o/keep reused, not re-fetched.
	if f.fetches-before != 1 {
		t.Errorf("stale install did %d fetches; want 1 (only changed o/a)", f.fetches-before)
	}
}

func TestInstallStaleReusedTamperedTreeHardError(t *testing.T) {
	// On the stale path, a reused-and-present module must still be digest-verified:
	// a tampered on-disk tree is a hard error, not silently trusted.
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "r1"}: hex40("a"), {"https://x/a.git", "r2"}: hex40("b"),
			{"https://x/k.git", "rk"}: hex40("c"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`},
			hex40("b"): {"wisp.json": `{"name":"a","main":"m.wisp"}`},
			hex40("c"): {"wisp.json": `{"name":"keep","main":"m.wisp"}`},
		},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r1"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Tamper the reused module's on-disk tree.
	writeFile(t, filepath.Join(dir, ".wisp", "modules", "o", "keep", "evil.wisp"), "tampered")
	// Bump o/a -> stale path; o/keep is reused-and-present and must be verified.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r2"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	err := installAt(dir, f, io.Discard)
	if err == nil {
		t.Fatal("expected a hard error: reused o/keep on-disk tree was tampered")
	}
	if !strings.Contains(err.Error(), "o/keep") || !strings.Contains(err.Error(), "does not match lock") {
		t.Errorf("error should name the tampered module and the digest mismatch, got: %v", err)
	}
}

func TestInstallStaleResolutionFailureKeepsExistingWisp(t *testing.T) {
	// A stale-path PRE-APPLY failure (here: resolving a bumped ref the fetcher cannot
	// satisfy) must leave a PRE-EXISTING .wisp/ untouched -- cleanupOnFail is a no-op
	// when .wisp/ existed before the command, and no live module tree is mutated before
	// the resolution error. Pins the wispExisted=true early-return branch for the stale
	// path (the mirror of TestInstallFailedFirstAddLeavesNoWispDir's wispExisted=false).
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "r1"}: hex40("a"),
			{"https://x/k.git", "rk"}: hex40("c"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A"},
			hex40("c"): {"wisp.json": `{"name":"keep","main":"m.wisp"}`, "m.wisp": "K"},
		},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r1"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	preLock, _ := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	preA, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "a", "m.wisp"))
	preKeep, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "keep", "m.wisp"))
	// Bump o/a -> r2, which the fetcher cannot resolve -> stale-path resolution failure.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r2"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected a resolution failure on the unresolvable bumped ref")
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp")); err != nil {
		t.Errorf("pre-existing .wisp/ removed after a stale-path resolution failure: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "a", "m.wisp")); !bytes.Equal(b, preA) {
		t.Errorf("existing o/a tree changed on a pre-apply resolution failure")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "keep", "m.wisp")); !bytes.Equal(b, preKeep) {
		t.Errorf("existing o/keep tree changed on a pre-apply resolution failure")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.lock")); !bytes.Equal(b, preLock) {
		t.Errorf("wisp.lock changed on a pre-apply resolution failure")
	}
}

func TestInstallStaleRefetchDigestMismatchHardError(t *testing.T) {
	// On the stale path, a REUSED module (commit+git unchanged) whose on-disk tree is
	// ABSENT is re-fetched into staging; if the re-fetched tree's digest disagrees with
	// the reused lock digest it is a hard error ("tampered lock?"), never a silent fixup.
	// This pins the stale re-fetch verify branch (distinct from the reused-and-PRESENT
	// tamper path in TestInstallStaleReusedTamperedTreeHardError). o/a is bumped only to
	// force the stale path; o/keep is the reused module whose tree is deleted.
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "r1"}: hex40("a"), {"https://x/a.git", "r2"}: hex40("b"),
			{"https://x/k.git", "rk"}: hex40("c"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A1"},
			hex40("b"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A2"},
			hex40("c"): {"wisp.json": `{"name":"keep","main":"m.wisp"}`, "m.wisp": "K"},
		},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r1"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Corrupt o/keep's locked digest (valid format, wrong value) and delete its on-disk
	// tree, so the stale install reuses o/keep, finds the tree absent, re-fetches it, and
	// the re-fetched (correct) digest mismatches the corrupted lock digest.
	lockPath := filepath.Join(dir, "wisp.lock")
	lf, err := readLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	e := lf.Modules["o/keep"]
	e.TreeDigest = "sha256:" + hex64("f")
	lf.Modules["o/keep"] = e
	writeLock(lockPath, lf)
	if err := os.RemoveAll(filepath.Join(dir, ".wisp", "modules", "o", "keep")); err != nil {
		t.Fatal(err)
	}
	// Bump o/a -> r2 to force the stale path; o/keep's pin is unchanged so it is reused.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r2"},"o/keep":{"git":"https://x/k.git","ref":"rk"}}}`)
	err = installAt(dir, f, io.Discard)
	if err == nil {
		t.Fatal("expected a hard error: re-fetched o/keep tree does not match the (corrupted) lock digest")
	}
	if !strings.Contains(err.Error(), "o/keep") || !strings.Contains(err.Error(), "does not match lock") {
		t.Errorf("error should name the module and the digest mismatch, got: %v", err)
	}
}

func TestInstallRefusesSymlinkedWispDir(t *testing.T) {
	// A symlinked .wisp could redirect writes outside the project; refuse up front.
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, ".wisp")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
	}
	if err := installAt(dir, f, io.Discard); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink-refusal error, got: %v", err)
	}
}

func TestInstallRefusesSymlinkedModulesSubdir(t *testing.T) {
	// A symlinked .wisp/modules (or .staging/.backup) could redirect writes too.
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := os.MkdirAll(filepath.Join(dir, ".wisp"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, ".wisp", "modules")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
	}
	if err := installAt(dir, f, io.Discard); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink-refusal error for .wisp/modules, got: %v", err)
	}
}

func TestInstallRefusesSymlinkedModuleOwnerDir(t *testing.T) {
	// A symlinked <owner> component under .wisp/modules (e.g. a committed
	// .wisp/modules/o -> /tmp) would let MkdirAll/Rename place the <repo> tree
	// outside the project; refuse it before any write or digest.
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := os.MkdirAll(filepath.Join(dir, ".wisp", "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, ".wisp", "modules", "o")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
	}
	if err := installAt(dir, f, io.Discard); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink-refusal error for .wisp/modules/o, got: %v", err)
	}
	// The symlink target must be untouched (no module tree placed through it).
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("symlink target should be untouched, got %d entries", len(entries))
	}
}

func TestRefuseSymlinkedDirSurfacesInspectError(t *testing.T) {
	// refuseSymlinkedDir must only treat os.ErrNotExist as "absent". A path that
	// exists but cannot be inspected (parent not a directory, so Lstat fails with
	// ENOTDIR) must surface an error, never silently pass the guard.
	base := t.TempDir()
	notDir := filepath.Join(base, "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Lstat("<file>/child") fails with ENOTDIR (not ErrNotExist).
	err := refuseSymlinkedDir(filepath.Join(notDir, "child"))
	if err == nil {
		t.Fatal("expected an inspect error, got nil (guard silently passed)")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("ENOTDIR must not be reported as absent, got: %v", err)
	}
}

func TestInstallStaleReusesParentWithTransitiveChild(t *testing.T) {
	// AC5 (reused TRANSITIVE EDGE path): a reused PARENT module's transitive child edge is
	// reconstructed from the CHILD's OWN oldLock entry (oldLock[childKey].Git/.Ref), NOT
	// from the parent. Graph: root -> o/parent ; o/parent -> o/child. After a first
	// install, bump an UNRELATED root dep (o/bump) so the install goes stale (re-resolve),
	// but o/parent's pin is unchanged -> o/parent is REUSED from the lock. resolveGraph must
	// then enqueue o/parent's child edge o/child using o/child's own lock entry's git/ref,
	// reuse o/child too (its pin is unchanged), and leave o/child placed with the correct
	// git/ref/commit. Assert: o/parent and o/child are NOT re-fetched (only the bumped
	// o/bump is), and o/child's lock entry retains its correct git/ref/commit.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/p.git", "rp"}:    hex40("a"),
			{"https://x/c.git", "rc"}:    hex40("b"),
			{"https://x/bump.git", "b1"}: hex40("c"),
			{"https://x/bump.git", "b2"}: hex40("d"),
		},
		trees: map[string]map[string]string{
			// parent declares a transitive child o/child; child has no deps.
			hex40("a"): {"wisp.json": `{"name":"parent","main":"m.wisp","dependencies":{"o/child":{"git":"https://x/c.git","ref":"rc"}}}`},
			hex40("b"): {"wisp.json": `{"name":"child","main":"m.wisp"}`},
			hex40("c"): {"wisp.json": `{"name":"bump","main":"m.wisp"}`},
			hex40("d"): {"wisp.json": `{"name":"bump","main":"m.wisp"}`},
		},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/parent":{"git":"https://x/p.git","ref":"rp"},"o/bump":{"git":"https://x/bump.git","ref":"b1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Bump ONLY o/bump (b1 -> b2). o/parent's pin is unchanged, so o/parent is reused; its
	// transitive child o/child must be reconstructed from o/child's OWN lock entry.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/parent":{"git":"https://x/p.git","ref":"rp"},"o/bump":{"git":"https://x/bump.git","ref":"b2"}}}`)
	before := f.fetches
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Only o/bump@b2 is fetched; o/parent and its transitive o/child are both reused.
	if f.fetches-before != 1 {
		t.Errorf("stale install did %d fetches; want 1 (only bumped o/bump; reused parent+transitive child)", f.fetches-before)
	}
	lf, err := readLock(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	child, ok := lf.Modules["o/child"]
	if !ok {
		t.Fatal("transitive child o/child missing from lock after reusing its parent")
	}
	if child.Git != "https://x/c.git" || child.Ref != "rc" || child.Commit != hex40("b") {
		t.Errorf("reused transitive child has wrong git/ref/commit: %+v", child)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "child", "wisp.json")); err != nil {
		t.Errorf("reused transitive child tree not present: %v", err)
	}
}

func TestInstallStaleSameCommitDifferentRefRewritesRef(t *testing.T) {
	// AC4/AC5 (reuse must not carry the stale ref): a re-pin where the dep's declared REF
	// CHANGES (r1 -> r2) but BOTH refs resolve to the SAME commit+git. The first install
	// locks o/r at ref "r1". Bumping wisp.json's ref to "r2" makes the lock STALE (the
	// locked ref "r1" no longer string-equals the manifest ref "r2"), so installAt takes the
	// stale/re-resolve path and o/r is REUSED (its commit+git are unchanged). The rebuilt
	// wisp.lock MUST record the NEW ref "r2" -- reuse reuses only the cached deps/treeDigest,
	// never the old lock entry's ref. After the rewrite the lock is consistent with
	// wisp.json again, so a THIRD install must do ZERO remote queries (no Resolve, Fetch, or
	// DefaultRef) -- proving the stale ref was not perpetuated.
	f := &countingFetcher{inner: &fakeFetcher{
		// Both r1 and r2 resolve to the SAME commit hex40("1").
		resolve: map[refKey]string{
			{"https://x/o/r.git", "r1"}: hex40("1"),
			{"https://x/o/r.git", "r2"}: hex40("1"),
		},
		trees: map[string]map[string]string{
			hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"},
		},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"r1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Sanity: first install locked ref "r1".
	if lf, err := readLock(filepath.Join(dir, "wisp.lock")); err != nil {
		t.Fatal(err)
	} else if lf.Modules["o/r"].Ref != "r1" {
		t.Fatalf("precondition: first install should lock ref r1; got %q", lf.Modules["o/r"].Ref)
	}
	// Re-pin: declared ref r1 -> r2 (same commit+git). This goes STALE (locked ref r1 !=
	// manifest ref r2) so installAt re-resolves and REUSES o/r (commit+git unchanged).
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"r2"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// The rewritten lock must record the NEW ref r2, the SAME commit hex40("1"), and the
	// same git -- NOT the stale ref r1.
	lf, err := readLock(filepath.Join(dir, "wisp.lock"))
	if err != nil {
		t.Fatal(err)
	}
	e := lf.Modules["o/r"]
	if e.Ref != "r2" {
		t.Errorf("reuse carried the stale ref: lock ref = %q, want r2", e.Ref)
	}
	if e.Commit != hex40("1") || e.Git != "https://x/o/r.git" {
		t.Errorf("reuse altered commit/git: %+v", e)
	}
	// The lock is now consistent with wisp.json (locked ref r2 == manifest ref r2), so a
	// third install must do ZERO remote queries.
	resolvesBefore := f.resolves
	fetchesBefore := f.fetches
	defaultRefsBefore := f.defaultRefs
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	if f.resolves != resolvesBefore {
		t.Errorf("post-rewrite install made %d Resolve calls; want 0 (lock must be consistent with wisp.json after recording the new ref)", f.resolves-resolvesBefore)
	}
	if f.fetches != fetchesBefore {
		t.Errorf("post-rewrite install made %d Fetch calls; want 0", f.fetches-fetchesBefore)
	}
	if f.defaultRefs != defaultRefsBefore {
		t.Errorf("post-rewrite install made %d DefaultRef calls; want 0", f.defaultRefs-defaultRefsBefore)
	}
}

func TestInstallConsistentMixedOrderingRevertsOnTamper(t *testing.T) {
	// AC13 (consistent path, mixed ordering): on a CONSISTENT lock, one module is ABSENT
	// (placed during apply) and a second module is PRESENT but TAMPERED (its digest no
	// longer matches the lock -> hard error). The error can fire AFTER the absent module
	// has already been placed and/or an unreachable dir pruned. The consistent path must
	// route every live-tree mutation through the same backup/restore mechanism, so on the
	// tamper error the already-placed absent module is restored to absent and any pruned
	// dir restored, leaving the project byte-identical. (Both modules are reachable from
	// the root in the lock; tampering one and deleting the other's on-disk tree creates the
	// "place one, then hard-error on the other" mixed ordering.)
	f := &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "ra"}: hex40("a"),
			{"https://x/b.git", "rb"}: hex40("b"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A"},
			hex40("b"): {"wisp.json": `{"name":"b","main":"m.wisp"}`, "m.wisp": "B"},
		},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"ra"},"o/b":{"git":"https://x/b.git","ref":"rb"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Snapshot the consistent post-install state (the byte-identical target after revert).
	preLock, _ := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	preB, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "b", "m.wisp"))
	// Make o/a ABSENT (a consistent install would re-place it from the locked commit) and
	// o/b PRESENT-but-TAMPERED (a consistent install hard-errors on the digest mismatch).
	if err := os.RemoveAll(filepath.Join(dir, ".wisp", "modules", "o", "a")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".wisp", "modules", "o", "b", "m.wisp"), "TAMPERED")
	// The lock is still byte-consistent with wisp.json, so this is the CONSISTENT path. It
	// re-places absent o/a, then hard-errors when o/b's recomputed digest mismatches.
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected a hard error on the tampered present module under a consistent lock")
	}
	// The already-placed absent module o/a must be restored to ABSENT (it was newly placed
	// during this failed apply), and wisp.lock left byte-identical. The tampered o/b is
	// left as the caller's pre-command on-disk bytes (the install does not "fix" tampering;
	// it restores to the pre-command state, which here is the tampered bytes the user had).
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "a")); !os.IsNotExist(err) {
		t.Errorf("newly-placed absent module o/a was not removed on revert: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "b", "m.wisp")); !bytes.Equal(b, []byte("TAMPERED")) {
		t.Errorf("o/b should be restored to its pre-command (tampered) bytes; got %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.lock")); !bytes.Equal(b, preLock) {
		t.Errorf("wisp.lock not byte-identical after consistent-path revert")
	}
	_ = preB // preB documents the untampered byte content; o/b restores to the pre-command (tampered) state.
}

func TestInstallFailedFirstAddLeavesNoWispDir(t *testing.T) {
	// .wisp/.gitignore rollback safety: a FAILED first-time install (no pre-existing
	// .wisp/) must leave NO .wisp/ directory behind -- no .wisp/.gitignore, no .staging,
	// no .backup. The fetch fails (no tree for the SHA), so installAt errors before any
	// successful apply, and the did-not-exist rollback removes the whole .wisp/.
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{}, // no tree -> Fetch fails
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	// Confirm .wisp/ did not exist before the command.
	if _, err := os.Stat(filepath.Join(dir, ".wisp")); !os.IsNotExist(err) {
		t.Fatalf("precondition: .wisp must not exist pre-command: %v", err)
	}
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected the first-time install to fail on the fetch error")
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp")); !os.IsNotExist(err) {
		t.Errorf("failed first-time install left a .wisp/ behind; want none: %v", err)
	}
}

func TestInstallFailureRestoresOverwrittenAndPrunedTrees(t *testing.T) {
	// AC13: a pre-existing module tree that a stale install would overwrite OR prune
	// must be restored byte-identical when the install fails DURING THE APPLY phase
	// (after staging + digest-verify succeed, while live module dirs are being
	// overwritten/pruned). Injecting the failure at fetch/staging time would never
	// exercise the live-tree backup/restore path -- so the failure is injected via the
	// `applyHook` seam (see install.go). CHOSEN APPROACH (a): the hook fires AFTER each
	// live-dir mutation carrying that mutation's KIND, and the test arms it to fail only
	// ONCE BOTH an overwrite-move-aside AND a prune-move-aside have occurred. A one-shot
	// "fail after the first mutation" hook could not guarantee both classes were mutated
	// before the failure (apply order is not fixed), so it could prove only ONE of
	// overwrite-restore / prune-restore. Gating the injected failure on having seen BOTH
	// kinds makes the test conclusively prove BOTH restoration classes regardless of apply
	// order.
	// Initial graph: root -> o/a@r1 + o/gone@rg. Second graph: root -> o/a@r2 (o/gone
	// dropped -> PRUNE move-aside; o/a -> OVERWRITE move-aside). Staging + verify of o/a@r2
	// succeed; the apply hook forces the failure only after both kinds have fired -> full
	// revert of both the overwritten and the pruned trees.
	f := &countingFetcher{inner: &fakeFetcher{
		resolve: map[refKey]string{
			{"https://x/a.git", "r1"}: hex40("a"), {"https://x/a.git", "r2"}: hex40("b"),
			{"https://x/g.git", "rg"}: hex40("e"),
		},
		trees: map[string]map[string]string{
			hex40("a"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A1"},
			hex40("b"): {"wisp.json": `{"name":"a","main":"m.wisp"}`, "m.wisp": "A2"},
			hex40("e"): {"wisp.json": `{"name":"gone","main":"m.wisp"}`, "m.wisp": "G"},
		},
	}}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r1"},"o/gone":{"git":"https://x/g.git","ref":"rg"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	// Snapshot the pre-command project state.
	preLock, _ := os.ReadFile(filepath.Join(dir, "wisp.lock"))
	preManifest, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	preA, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "a", "m.wisp"))
	preGone, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "gone", "m.wisp"))

	// Mutate the manifest to the second graph. Staging + verify of o/a@r2 SUCCEED (the
	// tree for hex40("b") is configured); the failure is injected mid-apply via the hook.
	writeFile(t, filepath.Join(dir, "wisp.json"),
		`{"name":"root","main":"main.wisp","dependencies":{"o/a":{"git":"https://x/a.git","ref":"r2"}}}`)
	// Arm the hook to fail only after BOTH an overwrite move-aside AND a prune move-aside
	// have happened, so both restoration classes are guaranteed exercised. installAt calls
	// applyHook(kind) after each live-dir mutation; production leaves it nil.
	var sawOverwrite, sawPrune bool
	applyHook = func(kind applyMutation) error {
		switch kind {
		case applyOverwrite:
			sawOverwrite = true
		case applyPrune:
			sawPrune = true
		}
		if sawOverwrite && sawPrune {
			return fmt.Errorf("injected mid-apply failure after both an overwrite and a prune")
		}
		return nil
	}
	t.Cleanup(func() { applyHook = nil })
	if err := installAt(dir, f, io.Discard); err == nil {
		t.Fatal("expected install failure")
	}
	if !sawOverwrite || !sawPrune {
		t.Fatalf("apply did not perform BOTH an overwrite and a prune before failing (overwrite=%v prune=%v); test cannot prove both restoration classes", sawOverwrite, sawPrune)
	}

	// The overwritten tree (o/a) and the would-be-pruned tree (o/gone) are restored
	// byte-identical, and wisp.lock is byte-identical. (wisp.json revert is the CLI's
	// job, Task 9.)
	if b, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "a", "m.wisp")); !bytes.Equal(b, preA) {
		t.Errorf("overwritten tree o/a not restored: got %q want %q", b, preA)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".wisp", "modules", "o", "gone", "m.wisp")); !bytes.Equal(b, preGone) {
		t.Errorf("pruned tree o/gone not restored: got %q want %q", b, preGone)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.lock")); !bytes.Equal(b, preLock) {
		t.Errorf("wisp.lock not restored byte-identical on failure")
	}
	_ = preManifest // manifest revert is asserted in the Task 9 CLI atomicity test
}

func TestInstallWarnsOnCompilerMismatch(t *testing.T) {
	// Modeled on TestInstallConsistentLockNoFetch: build a project whose wisp.lock
	// is CONSISTENT with wisp.json, then stamp the on-disk lock with a known,
	// non-dev "wisp" value so the mismatch path is exercised deterministically.
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/o/r.git", "v1"}: hex40("1")},
		trees:   map[string]map[string]string{hex40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`)
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}

	lockPath := filepath.Join(dir, "wisp.lock")
	lf, err := readLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	lf.Wisp = "1.0.0"
	stampedBytes, err := marshalLock(lf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, stampedBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	origVersion := version.Number
	defer func() { version.Number = origVersion }()

	version.Number = "2.0.0"
	var buf bytes.Buffer
	if err := installAt(dir, f, &buf); err != nil {
		t.Fatalf("install failed despite a compiler-version mismatch (criterion #4): %v", err)
	}
	out := buf.String()
	if strings.Count(out, "wisp: warning:") != 1 {
		t.Errorf("want exactly one warning line, got %q", out)
	}
	if !strings.Contains(out, "1.0.0") || !strings.Contains(out, "2.0.0") {
		t.Errorf("warning does not name both versions: %q", out)
	}
	lockAfter, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(lockAfter, stampedBytes) {
		t.Errorf("consistent install with a mismatch warning rewrote wisp.lock; bytes must be unchanged")
	}

	// Companion: with version.Number at the dev sentinel, the same install emits
	// no warning (dev-sentinel suppression through the real install path).
	version.Number = "0.0.0-dev"
	var buf2 bytes.Buffer
	if err := installAt(dir, f, &buf2); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf2.String(), "wisp: warning:") {
		t.Errorf("dev-sentinel running version must suppress the warning; got %q", buf2.String())
	}
}

func TestInstallThenModuleLoadResolves(t *testing.T) {
	f := &fakeFetcher{
		resolve: map[refKey]string{{"https://x/owner/repo.git", "v1"}: hex40("1")},
		trees: map[string]map[string]string{hex40("1"): {
			"wisp.json": `{"name":"repo","main":"lib.wisp"}`,
			"lib.wisp":  "export fn get() -> int { return 1 }\n"}},
	}
	dir := project(t, `{"name":"root","main":"main.wisp","dependencies":{"owner/repo":{"git":"https://x/owner/repo.git","ref":"v1"}}}`)
	// Root imports the package.
	writeFile(t, filepath.Join(dir, "main.wisp"), "import \"owner/repo\"\nfn main() -> int { return repo.get() }\n")
	if err := installAt(dir, f, io.Discard); err != nil {
		t.Fatal(err)
	}
	rootFile := filepath.Join(dir, "main.wisp")
	b, _ := os.ReadFile(rootFile)
	linked, diags := module.Load(rootFile, string(b), module.OSFS{})
	if len(diags) > 0 {
		t.Fatalf("module.Load diags after install: %v", diags)
	}
	if _, ok := linked.Modules[0].Namespaces["repo"]; !ok {
		t.Errorf("import owner/repo did not resolve; namespaces=%v", linked.Modules[0].Namespaces)
	}
}
