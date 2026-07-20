package module

import (
	"path/filepath"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

func TestSplitRepoExported(t *testing.T) {
	cases := []struct {
		in          string
		owner, repo string
		wantErr     bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"a/b", "a", "b", false},
		{"a", "", "", true},       // one segment
		{"a/b/c", "", "", true},   // three segments
		{"../b", "", "", true},    // dotdot segment
		{"owner/.", "", "", true}, // dot segment
		{"o/r$", "", "", true},    // bad charset
	}
	for _, c := range cases {
		owner, repo, err := SplitRepo(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("SplitRepo(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && (owner != c.owner || repo != c.repo) {
			t.Errorf("SplitRepo(%q) = %q,%q want %q,%q", c.in, owner, repo, c.owner, c.repo)
		}
	}
}

func TestWithinExported(t *testing.T) {
	if !Within("/a/b/c", "/a/b") {
		t.Error("Within(/a/b/c, /a/b) = false, want true")
	}
	if Within("/a/x", "/a/b") {
		t.Error("Within(/a/x, /a/b) = true, want false")
	}
}

func TestDiscoverRootExported(t *testing.T) {
	dir := proj(t, map[string]string{
		"wisp.json":    `{"name":"root","main":"main.wisp"}`,
		"sub/keep.txt": "x",
	})
	got := DiscoverRoot(OSFS{}, filepath.Join(dir, "sub"))
	want := filepath.Join(dir)
	if got != want {
		t.Errorf("DiscoverRoot = %q, want %q", got, want)
	}
}

// TestLoaderParityExportedHelpers proves AC14 concretely: the EXPORTED SplitRepo helper
// the package tool consumes produces IDENTICAL accept/reject verdicts to the loader's own
// private splitRepo path that it backs, over a shared fixture set. This proves the
// reused-helper parity claim, not merely that the helper was exported.
//
// Accept/reject parity: SplitRepo and the loader's splitRepo method are the SAME
// implementation (splitRepo delegates to SplitRepo and turns the error into a diag), so
// for every owner/repo string the EXPORTED helper's accept/reject verdict must equal the
// loader path's verdict (a non-nil error iff the loader emits a diagnostic).
//
// (Root discovery is no longer parity-tested here: the loader calls DiscoverRoot directly
// with no private wrapper to diverge; DiscoverRoot's behavior is covered by
// TestDiscoverRootExported.)
func TestLoaderParityExportedHelpers(t *testing.T) {
	l := &loader{fs: OSFS{}, byPath: map[string]*Module{}}

	// Shared accept/reject fixture set of owner/repo strings; the exported SplitRepo
	// verdict must equal the loader splitRepo verdict for each.
	for _, s := range []string{"owner/repo", "a/b", "a", "a/b/c", "../b", "owner/.", "o/r$", "a/-x", "ok_seg/ok-seg.v1"} {
		_, _, exErr := SplitRepo(s)
		// token.Position here is the PROJECT-INTERNAL token type
		// (github.com/mitchellnemitz/wisp/internal/token), NOT stdlib go/token --
		// the loader's splitRepo signature takes the internal Position.
		_, _, loaderOK := l.splitRepo(s, token.Position{})
		if (exErr == nil) != loaderOK {
			t.Errorf("accept/reject parity for %q: SplitRepo err=%v but loader splitRepo ok=%v", s, exErr, loaderOK)
		}
	}
}
