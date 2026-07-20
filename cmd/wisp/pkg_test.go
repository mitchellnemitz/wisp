package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/pkgtool"
)

// cliFake is a minimal pkgtool.Fetcher for CLI tests. The cmd package cannot
// import the pkgtool-internal test fake, so we define one here.
type cliFake struct {
	resolve    map[[2]string]string
	trees      map[string]map[string]string
	defaultRef map[string]string
}

var _ pkgtool.Fetcher = (*cliFake)(nil)

func (c *cliFake) DefaultRef(url string) (string, error) {
	if r, ok := c.defaultRef[url]; ok {
		return r, nil
	}
	return "", os.ErrNotExist
}
func (c *cliFake) Resolve(url, ref string) (string, error) {
	if s, ok := c.resolve[[2]string{url, ref}]; ok {
		return s, nil
	}
	return "", os.ErrNotExist
}
func (c *cliFake) Fetch(url, ref, sha, dest string) error {
	tree, ok := c.trees[sha]
	if !ok {
		return fmt.Errorf("cliFake: no tree for sha %s", sha)
	}
	for rel, content := range tree {
		full := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func sha40(c string) string { return strings.Repeat(c, 40) }

func TestAddWritesManifestAndLock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{{"https://x/o/r.git", "v1"}: sha40("1")},
		trees:   map[string]map[string]string{sha40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}
	var so, se bytes.Buffer
	code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f)
	if code != 0 {
		t.Fatalf("add exit=%d stderr=%q", code, se.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	if !bytes.Contains(b, []byte(`"o/r"`)) {
		t.Errorf("dependency not written: %s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "wisp.lock")); err != nil {
		t.Errorf("lock not written: %v", err)
	}
}

func TestAddFirstWithoutGitUsageError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--ref", "v1"}, &so, &se, &cliFake{}); code != 2 {
		t.Fatalf("first add without --git: exit=%d want 2", code)
	}
}

func TestAddInvalidRepoIsUsageErrorAndDoesNotTouchManifest(t *testing.T) {
	dir := t.TempDir()
	before := `{"name":"root","main":"main.wisp"}`
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(before), 0o644)
	var so, se bytes.Buffer
	// A malformed key (not exactly owner/repo) must be a usage error (exit 2),
	// raised before wisp.json is rewritten.
	if code := pkgRun(dir, "add", []string{"not-a-repo", "--git", "https://x/o/r.git"}, &so, &se, &cliFake{}); code != 2 {
		t.Fatalf("invalid repo: exit=%d want 2 (stderr=%q)", code, se.String())
	}
	got, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	if string(got) != before {
		t.Errorf("wisp.json was modified on an invalid-repo usage error:\n got=%q\nwant=%q", got, before)
	}
}

func TestAddFirstNoRefRecordsDefaultBranchName(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		defaultRef: map[string]string{"https://x/o/r.git": "main"},
		resolve:    map[[2]string]string{{"https://x/o/r.git", "main"}: sha40("1")},
		trees:      map[string]map[string]string{sha40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`}},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git"}, &so, &se, f); code != 0 {
		t.Fatalf("no-ref first add exit=%d stderr=%q", code, se.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	if bytes.Contains(b, []byte(`"HEAD"`)) {
		t.Errorf("recorded literal HEAD; want the concrete default branch name: %s", b)
	}
	if !bytes.Contains(b, []byte(`"ref": "main"`)) && !bytes.Contains(b, []byte(`"ref":"main"`)) {
		t.Errorf("did not record default branch name 'main': %s", b)
	}
}

func TestAddRepinNoGitReusesStoredURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{
			{"https://x/o/r.git", "v1"}: sha40("1"),
			{"https://x/o/r.git", "v2"}: sha40("2"),
		},
		trees: map[string]map[string]string{
			sha40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`},
			sha40("2"): {"wisp.json": `{"name":"r","main":"m.wisp"}`},
		},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f); code != 0 {
		t.Fatalf("initial add exit=%d stderr=%q", code, se.String())
	}
	if code := pkgRun(dir, "add", []string{"o/r", "--ref", "v2"}, &so, &se, f); code != 0 {
		t.Fatalf("re-pin add exit=%d stderr=%q", code, se.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	if !bytes.Contains(b, []byte("https://x/o/r.git")) {
		t.Errorf("re-pin dropped the stored git URL: %s", b)
	}
	if !bytes.Contains(b, []byte(`"v2"`)) {
		t.Errorf("re-pin did not record the new ref v2: %s", b)
	}
}

func TestAddRepinNoRefReusesExistingRef(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		defaultRef: map[string]string{"https://x/o/r.git": "main"},
		resolve: map[[2]string]string{
			{"https://x/o/r.git", "v1"}:   sha40("1"),
			{"https://x/o/r.git", "main"}: sha40("9"),
		},
		trees: map[string]map[string]string{
			sha40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`},
			sha40("9"): {"wisp.json": `{"name":"r","main":"m.wisp"}`},
		},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f); code != 0 {
		t.Fatalf("initial add exit=%d stderr=%q", code, se.String())
	}
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git"}, &so, &se, f); code != 0 {
		t.Fatalf("re-pin add exit=%d stderr=%q", code, se.String())
	}
	b, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	if bytes.Contains(b, []byte(`"ref": "main"`)) || bytes.Contains(b, []byte(`"ref":"main"`)) {
		t.Errorf("re-pin with no --ref switched to the default branch; must reuse v1: %s", b)
	}
	if !bytes.Contains(b, []byte(`"v1"`)) {
		t.Errorf("re-pin with no --ref did not reuse the existing ref v1: %s", b)
	}
}

func TestAddFetchFailureNamesRepoAndURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{{"https://x/o/r.git", "v1"}: sha40("1")},
		trees:   map[string]map[string]string{},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f); code == 0 {
		t.Fatal("expected add to fail on the fetch error")
	}
	if !bytes.Contains(se.Bytes(), []byte("o/r")) || !bytes.Contains(se.Bytes(), []byte("https://x/o/r.git")) {
		t.Errorf("failure message must name BOTH owner/repo and URL; got %q", se.String())
	}
}

func TestInstallFetchFailureNamesRepoAndURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"),
		[]byte(`{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{{"https://x/o/r.git", "v1"}: sha40("1")},
		trees:   map[string]map[string]string{},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "install", nil, &so, &se, f); code == 0 {
		t.Fatal("expected install to fail on the fetch error")
	}
	if !bytes.Contains(se.Bytes(), []byte("o/r")) || !bytes.Contains(se.Bytes(), []byte("https://x/o/r.git")) {
		t.Errorf("install failure message must name BOTH owner/repo and URL; got %q", se.String())
	}
}

func TestRemoveFetchFailureNamesRepoAndURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{
			{"https://x/o/keep.git", "v1"}: sha40("1"),
			{"https://x/o/drop.git", "v1"}: sha40("2"),
		},
		trees: map[string]map[string]string{
			sha40("1"): {"wisp.json": `{"name":"keep","main":"m.wisp"}`},
			sha40("2"): {"wisp.json": `{"name":"drop","main":"m.wisp"}`},
		},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/keep", "--git", "https://x/o/keep.git", "--ref", "v1"}, &so, &se, f); code != 0 {
		t.Fatalf("add keep exit=%d stderr=%q", code, se.String())
	}
	if code := pkgRun(dir, "add", []string{"o/drop", "--git", "https://x/o/drop.git", "--ref", "v1"}, &so, &se, f); code != 0 {
		t.Fatalf("add drop exit=%d stderr=%q", code, se.String())
	}
	if err := os.RemoveAll(filepath.Join(dir, ".wisp", "modules", "o", "keep")); err != nil {
		t.Fatal(err)
	}
	delete(f.trees, sha40("1"))
	se.Reset()
	if code := pkgRun(dir, "remove", []string{"o/drop"}, &so, &se, f); code == 0 {
		t.Fatal("expected remove to fail when the re-resolve cannot fetch the still-present dep")
	}
	if !bytes.Contains(se.Bytes(), []byte("o/keep")) || !bytes.Contains(se.Bytes(), []byte("https://x/o/keep.git")) {
		t.Errorf("remove failure message must name BOTH owner/repo and URL; got %q", se.String())
	}
}

func TestAddDefaultRefDiscoveryFailureNamesRepoAndURL(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git"}, &so, &se, f); code == 0 {
		t.Fatal("expected add to fail when the default branch cannot be discovered")
	}
	if !bytes.Contains(se.Bytes(), []byte("o/r")) || !bytes.Contains(se.Bytes(), []byte("https://x/o/r.git")) {
		t.Errorf("default-branch discovery failure must name BOTH owner/repo and URL; got %q", se.String())
	}
}

func TestAddFailureRevertsManifestAndLockAndModules(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	preManifest, _ := os.ReadFile(filepath.Join(dir, "wisp.json"))
	f := &cliFake{
		resolve: map[[2]string]string{{"https://x/o/r.git", "v1"}: sha40("1")},
		trees:   map[string]map[string]string{},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f); code == 0 {
		t.Fatal("expected add to fail when the fetch fails")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.json")); !bytes.Equal(b, preManifest) {
		t.Errorf("wisp.json not reverted: got %q want %q", b, preManifest)
	}
	if _, err := os.Stat(filepath.Join(dir, "wisp.lock")); !os.IsNotExist(err) {
		t.Errorf("wisp.lock written despite failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r")); !os.IsNotExist(err) {
		t.Errorf("partial module tree left behind: %v", err)
	}
}

func TestRemoveDeletesEntryPrunesAndUpdatesLock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.wisp"), []byte("fn main() -> int { return 0 }\n"), 0o644)
	f := &cliFake{
		resolve: map[[2]string]string{{"https://x/o/r.git", "v1"}: sha40("1")},
		trees:   map[string]map[string]string{sha40("1"): {"wisp.json": `{"name":"r","main":"m.wisp"}`, "m.wisp": "x"}},
	}
	var so, se bytes.Buffer
	if code := pkgRun(dir, "add", []string{"o/r", "--git", "https://x/o/r.git", "--ref", "v1"}, &so, &se, f); code != 0 {
		t.Fatalf("add exit=%d stderr=%q", code, se.String())
	}
	if code := pkgRun(dir, "remove", []string{"o/r"}, &so, &se, f); code != 0 {
		t.Fatalf("remove exit=%d stderr=%q", code, se.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.json")); bytes.Contains(b, []byte(`"o/r"`)) {
		t.Errorf("dependency entry not deleted: %s", b)
	}
	if _, err := os.Stat(filepath.Join(dir, ".wisp", "modules", "o", "r")); !os.IsNotExist(err) {
		t.Errorf("module tree not pruned: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wisp.lock")); bytes.Contains(b, []byte(`"o/r"`)) {
		t.Errorf("lock still references removed key: %s", b)
	}
}

func TestRemoveAbsentKeyHardError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"), []byte(`{"name":"root","main":"main.wisp"}`), 0o644)
	var so, se bytes.Buffer
	code := pkgRun(dir, "remove", []string{"o/r"}, &so, &se, &cliFake{})
	if code == 0 {
		t.Fatal("removing an absent key must be a hard error")
	}
	if !bytes.Contains(se.Bytes(), []byte("o/r")) {
		t.Errorf("remove error should name the key: %q", se.String())
	}
}

func TestUnknownPkgSubcommandStillExit2(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"frobnicate"}, &so, &se); code != 2 {
		t.Fatalf("unknown command exit=%d want 2", code)
	}
}

func TestBuildRunCheckDoNotInvokePkgtool(t *testing.T) {
	// Build a project that has a wisp.json with dependencies but an empty
	// .wisp/modules: build of a non-importing main must succeed WITHOUT any fetch.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wisp.json"),
		[]byte(`{"name":"root","main":"main.wisp","dependencies":{"o/r":{"git":"https://x/o/r.git","ref":"v1"}}}`), 0o644)
	src := filepath.Join(dir, "main.wisp")
	os.WriteFile(src, []byte("fn main() -> int { return 0 }\n"), 0o644)
	var so, se bytes.Buffer
	// build/run/check take a file path, not the pkg subcommands; no fetcher is
	// constructed on these paths at all. `run` is exercised too because Task 9 edits the
	// same top-level dispatch (`switch cmd`) where `run` could be miswired into pkgRun.
	if code := run([]string{"check", src}, &so, &se); code != 0 {
		t.Fatalf("check exit=%d stderr=%q", code, se.String())
	}
	out := filepath.Join(dir, "out.sh")
	if code := run([]string{"build", src, "-o", out}, &so, &se); code != 0 {
		t.Fatalf("build exit=%d stderr=%q", code, se.String())
	}
	// `wisp run` of the non-importing main compiles and executes with no fetch; the
	// program returns 0. (This guards against Task 9's dispatch edits routing `run`
	// through the package tool.)
	if code := run([]string{"run", src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%q", code, se.String())
	}
}
