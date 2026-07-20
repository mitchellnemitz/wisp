package pkgtool

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitOrSkip(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

// gitRun runs git in repo with a fixed author/committer identity and
// neutralized signing config so the test is hermetic w.r.t. the user's
// global ~/.gitconfig (e.g. tag.gpgsign=true or commit.gpgsign=true).
func gitRun(t *testing.T, repo string, args ...string) string {
	t.Helper()
	// Prepend -c flags before the subcommand so they apply globally.
	full := append([]string{
		"-c", "tag.gpgsign=false",
		"-c", "commit.gpgsign=false",
		"-c", "core.autocrlf=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// makeBareRepo creates a SERVABLE remote (a real --bare repo) and returns its file://
// URL, the name of a branch whose TIP is a descendant of the pinned commit, and the
// pinned commit SHA.
//
// The pinned commit is deliberately a NON-TIP, reachable commit: we commit the TARGET
// (carrying wisp.json + m.wisp), capture its SHA, then add at least one MORE commit on
// the SAME branch. The branch tip is therefore a DESCENDANT of the target, and the
// target SHA is reachable from the branch tip but is NOT itself an advertised ref tip
// (no branch head and no tag points AT it). Combined with the bare remote's default
// uploadpack config (allowAnySHA1InWant / allowTipSHA1InWant / allowReachableSHA1InWant
// all OFF), `git fetch <url> <targetSHA>` is GENUINELY REFUSED -- even over file://,
// where a tip-advertised SHA might otherwise be want-able. Only the ref-tip /
// all-refs fallback, which fetches a REF and walks its history to reach the ancestor
// target, can obtain the commit. This makes both fallback tests deterministic instead of
// relying on the target SHA happening not to be advertised.
//
// Why a non-tip target: if the pinned commit WERE an advertised ref tip (as in the old
// fixture, which tagged the exact fetched commit), some git versions/transports
// (notably file://) may satisfy the bare-SHA want directly, so the initial
// `fetch <sha>` would succeed and the fallback would NOT run -- making the trace
// assertions nondeterministic. A non-tip target forecloses that.
//
// HEAD repoint: after pushing wisptest, the bare remote's symbolic HEAD still points at
// its init default (master/main), which was never pushed -- so `ls-remote --symref HEAD`
// returns an empty symref and DefaultRef would hard-error. The helper repoints the bare
// HEAD at refs/heads/wisptest so the remote advertises wisptest as its default branch.
// wisptest remains a real pushed ref, so Resolve(url, "wisptest") and the Fetch ref-tip
// fallback are unaffected.
func makeBareRepo(t *testing.T) (url, branch, sha string) {
	t.Helper()
	work := t.TempDir()
	gitRun(t, work, "init", "-q")
	// Pin the repo's default branch name so the returned branch is stable across git
	// versions (older git defaults to "master", newer to "main").
	gitRun(t, work, "checkout", "-q", "-b", "wisptest")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(work, "m.wisp"), []byte("export fn x() -> int { return 0 }\n"), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "target")
	// Capture the TARGET commit SHA -- this is the pinned, non-tip commit.
	sha = strings.TrimSpace(gitRun(t, work, "rev-parse", "HEAD"))
	// Add a LATER commit on the same branch so the tip is a descendant of the target and
	// the target is no longer the branch tip (and is pointed at by no tag).
	os.WriteFile(filepath.Join(work, "later.txt"), []byte("advance the tip\n"), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "advance")

	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q")
	// Belt-and-suspenders: explicitly keep arbitrary/tip/reachable-SHA wants OFF (the
	// defaults), so a bare-SHA want for the non-tip target is refused.
	gitRun(t, bare, "config", "uploadpack.allowAnySHA1InWant", "false")
	gitRun(t, bare, "config", "uploadpack.allowTipSHA1InWant", "false")
	gitRun(t, bare, "config", "uploadpack.allowReachableSHA1InWant", "false")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*")
	// Repoint the bare remote's symbolic HEAD at the PUSHED branch. A `git init --bare`
	// sets HEAD to the init default (master/main), and that default ref is NEVER pushed
	// here -- so `git ls-remote --symref <bare> HEAD` would otherwise return an empty
	// symref line and DefaultRef would hard-error. Pointing HEAD at refs/heads/wisptest
	// makes the bare remote advertise wisptest as its default branch.
	gitRun(t, bare, "symbolic-ref", "HEAD", "refs/heads/wisptest")
	return "file://" + bare, "wisptest", sha
}

func TestGitFetcherDefaultRef(t *testing.T) {
	gitOrSkip(t)
	url, _, _ := makeBareRepo(t)
	gf := gitFetcher{}
	ref, err := gf.DefaultRef(url)
	if err != nil {
		t.Fatal(err)
	}
	// makeBareRepo repoints the bare remote's HEAD at refs/heads/wisptest, so DefaultRef
	// must return the actual default-branch NAME "wisptest" -- not merely a non-empty,
	// non-"HEAD" string.
	if ref != "wisptest" {
		t.Errorf("DefaultRef = %q; want the concrete default-branch name %q", ref, "wisptest")
	}
}

// fileURLAllowsBareSHAFetch probes whether the local git allows bare-SHA fetches
// over file:// transport regardless of uploadpack.allow*SHA1InWant config. Git
// 2.33+ (and some earlier builds) bypass those restrictions for local transports.
// When this returns true the ladder-step assertions in fallback tests are skipped
// (the outcome assertions -- tree materialization -- still run).
func fileURLAllowsBareSHAFetch(t *testing.T) bool {
	t.Helper()
	work := t.TempDir()
	bare := t.TempDir()
	gitRun(t, work, "init", "-q")
	gitRun(t, work, "checkout", "-q", "-b", "probe")
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "target")
	targetSHA := strings.TrimSpace(gitRun(t, work, "rev-parse", "HEAD"))
	// Advance so targetSHA is no longer the tip.
	if err := os.WriteFile(filepath.Join(work, "g"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "advance")
	gitRun(t, bare, "init", "--bare", "-q")
	gitRun(t, bare, "config", "uploadpack.allowAnySHA1InWant", "false")
	gitRun(t, bare, "config", "uploadpack.allowTipSHA1InWant", "false")
	gitRun(t, bare, "config", "uploadpack.allowReachableSHA1InWant", "false")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*")
	fetch := t.TempDir()
	gitRun(t, fetch, "init", "-q")
	cmd := exec.Command("git", "-C", fetch, "fetch", "--depth", "1", "--", "file://"+bare, targetSHA)
	return cmd.Run() == nil
}

func TestGitFetcherResolveAndFetchSHARefusalFallback(t *testing.T) {
	gitOrSkip(t)
	// branch's TIP is a DESCENDANT of the pinned target sha; the target is NOT an
	// advertised ref tip, so a bare-SHA want for it is genuinely refused (on network
	// transports) and only the ref-tip fallback can obtain it. Over file://, some git
	// versions bypass uploadpack.allow*SHA1InWant and the sha is fetchable directly;
	// in that case only the outcome (tree materialised) is asserted.
	url, branch, sha := makeBareRepo(t)
	gf := gitFetcher{}
	if _, err := gf.Resolve(url, branch); err != nil {
		t.Fatalf("resolve branch %q: %v", branch, err)
	}

	var trace []fetchStep
	fetchTrace = func(s fetchStep) { trace = append(trace, s) }
	t.Cleanup(func() { fetchTrace = nil })

	dest := t.TempDir()
	if err := gf.Fetch(url, branch, sha, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "wisp.json")); err != nil {
		t.Errorf("fetch did not materialize tree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git must not be copied: %v", err)
	}
	// The ladder-step assertions only hold when file:// actually refuses bare-SHA wants.
	if fileURLAllowsBareSHAFetch(t) {
		t.Log("file:// bypasses uploadpack SHA restrictions on this git; skipping ladder-step assertions")
		return
	}
	var sawSHAAttemptFailed, sawRefTip bool
	for _, s := range trace {
		if s.kind == fetchSHA && s.err != nil {
			sawSHAAttemptFailed = true
		}
		if s.kind == fetchRefTip {
			sawRefTip = true
		}
	}
	if !sawSHAAttemptFailed {
		t.Errorf("expected the initial bare-SHA fetch to be attempted and REFUSED; trace=%+v", trace)
	}
	if !sawRefTip {
		t.Errorf("expected the ref-tip fallback to run; trace=%+v", trace)
	}
}

func TestGitFetcherSHAValuedRefAllRefsFallback(t *testing.T) {
	gitOrSkip(t)
	// SHA-VALUED-ref special case: the recorded ref IS itself the full commit SHA, so
	// there is no symbolic tip to fall back to. The target is a NON-TIP commit. Against a
	// remote that refuses bare-SHA wants, Fetch must escalate to +refs/*:refs/*. Over
	// file://, some git versions bypass uploadpack restrictions; the all-refs fallback may
	// or may not run, but the outcome (tree materialised) must always hold.
	url, _, sha := makeBareRepo(t)
	gf := gitFetcher{}

	var trace []fetchStep
	fetchTrace = func(s fetchStep) { trace = append(trace, s) }
	t.Cleanup(func() { fetchTrace = nil })

	dest := t.TempDir()
	// ref == sha (a full SHA), so the SHA-valued-ref branch in Fetch is taken.
	if err := gf.Fetch(url, sha, sha, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "wisp.json")); err != nil {
		t.Errorf("fetch did not materialize tree: %v", err)
	}
	if fileURLAllowsBareSHAFetch(t) {
		t.Log("file:// bypasses uploadpack SHA restrictions on this git; all-refs fallback may not run")
		return
	}
	var sawAllRefs bool
	for _, s := range trace {
		if s.kind == fetchAllRefs {
			sawAllRefs = true
		}
	}
	if !sawAllRefs {
		t.Errorf("expected the SHA-valued-ref all-refs (+refs/*:refs/*) fallback to run; trace=%+v", trace)
	}
}

func TestGitFetcherResolveAnnotatedTagResolves(t *testing.T) {
	gitOrSkip(t)
	// An ANNOTATED tag makes ls-remote emit BOTH refs/tags/at and refs/tags/at^{}.
	// That is NOT ambiguity: it must resolve to the peeled commit, not error.
	work := t.TempDir()
	gitRun(t, work, "init", "-q")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "init")
	gitRun(t, work, "tag", "-a", "at", "-m", "annotated")
	wantCommit := strings.TrimSpace(gitRun(t, work, "rev-parse", "at^{commit}"))
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/tags/*:refs/tags/*", "refs/heads/*:refs/heads/*")

	gf := gitFetcher{}
	got, err := gf.Resolve("file://"+bare, "at")
	if err != nil {
		t.Fatalf("annotated tag must resolve, got error: %v", err)
	}
	if got != wantCommit {
		t.Errorf("annotated tag resolved to %s; want peeled commit %s", got, wantCommit)
	}
}

func TestGitFetcherResolveAmbiguousIsError(t *testing.T) {
	gitOrSkip(t)
	// A repo with BOTH a branch and a tag named "dup" => two DISTINCT non-peeled
	// refnames match => hard ambiguity error.
	work := t.TempDir()
	gitRun(t, work, "init", "-q")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "init")
	gitRun(t, work, "branch", "dup")
	gitRun(t, work, "tag", "dup")
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*")

	gf := gitFetcher{}
	if _, err := gf.Resolve("file://"+bare, "dup"); err == nil {
		t.Fatal("tag-vs-branch ambiguity must be a hard error")
	}
}

func TestGitFetcherResolveNoMatchIsError(t *testing.T) {
	gitOrSkip(t)
	work := t.TempDir()
	gitRun(t, work, "init", "-q")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "init")
	gitRun(t, work, "tag", "v1")
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*")

	gf := gitFetcher{}
	if _, err := gf.Resolve("file://"+bare, "no-such-ref-v999"); err == nil {
		t.Fatal("ls-remote matching no ref must be a hard not-found error")
	}
}

func TestGitFetcherDeterministicExtractionHostileGitattributes(t *testing.T) {
	gitOrSkip(t)
	// Commit a file with CRLF content and a .gitattributes forcing text=auto eol=crlf;
	// fetch and assert the materialized bytes equal the committed blob bytes (no
	// line-ending conversion).
	work := t.TempDir()
	gitRun(t, work, "init", "-q")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(work, ".gitattributes"), []byte("* text=auto eol=crlf\n"), 0o644)
	crlf := []byte("line1\r\nline2\r\n")
	os.WriteFile(filepath.Join(work, "m.wisp"), crlf, 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "init")
	gitRun(t, work, "tag", "v1")
	sha := strings.TrimSpace(gitRun(t, work, "rev-parse", "v1"))
	wantBlob := []byte(gitRun(t, work, "cat-file", "blob", "v1:m.wisp"))
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*")

	gf := gitFetcher{}
	dest := t.TempDir()
	if err := gf.Fetch("file://"+bare, "v1", sha, dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "m.wisp"))
	if !bytes.Equal(got, wantBlob) {
		t.Errorf("materialized bytes differ from committed blob; git config altered the tree\n got=%q\nwant=%q", got, wantBlob)
	}
}

func TestGitFetcherSHA256Repo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	if !gitSupportsSHA256() {
		t.Skip("git lacks --object-format=sha256 support")
	}
	// A SHA-256 remote yields a 64-hex commit; Fetch must init a sha256 temp repo
	// (inferred from the 64-hex sha) so the fetch does not fail on object-format mismatch.
	work := t.TempDir()
	gitRun(t, work, "init", "-q", "--object-format=sha256")
	os.WriteFile(filepath.Join(work, "wisp.json"), []byte(`{"name":"r","main":"m.wisp"}`), 0o644)
	os.WriteFile(filepath.Join(work, "m.wisp"), []byte("x"), 0o644)
	gitRun(t, work, "add", ".")
	gitRun(t, work, "commit", "-q", "-m", "init")
	gitRun(t, work, "branch", "-M", "wisptest")
	sha := strings.TrimSpace(gitRun(t, work, "rev-parse", "HEAD"))
	if len(sha) != 64 {
		t.Fatalf("expected a 64-hex sha256 commit, got %q (len %d)", sha, len(sha))
	}
	bare := t.TempDir()
	gitRun(t, bare, "init", "--bare", "-q", "--object-format=sha256")
	gitRun(t, work, "push", "-q", "file://"+bare, "refs/heads/*:refs/heads/*")
	gitRun(t, bare, "symbolic-ref", "HEAD", "refs/heads/wisptest")

	gf := gitFetcher{}
	dest := t.TempDir()
	if err := gf.Fetch("file://"+bare, "wisptest", sha, dest); err != nil {
		t.Fatalf("Fetch of a sha256 pin failed (object-format not negotiated?): %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "m.wisp")); string(b) != "x" {
		t.Errorf("sha256 tree not materialized: got %q", b)
	}
}

// gitSupportsSHA256 reports whether the local git can init a sha256-object-format repo.
func gitSupportsSHA256() bool {
	d, err := os.MkdirTemp("", "wisp-sha256probe-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(d)
	return exec.Command("git", "-C", d, "init", "--object-format=sha256", "-q").Run() == nil
}
