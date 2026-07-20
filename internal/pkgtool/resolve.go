package pkgtool

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mitchellnemitz/wisp/internal/module"
)

type resolved struct {
	git, ref, commit, treeDigest string
	deps                         []string
	dir                          string // staged temp dir holding the fetched tree (filled when fetched fresh)
}

// pin is a (commit,url) request for a key, tagged with the requesting parent and
// whether it came from the root.
type pin struct {
	commit, url, ref string
	parent           string // "<root>" for a root dep
	isRoot           bool
}

// resolveGraph performs the section-4.4 walk. stagingRoot is a caller-owned directory
// (always <root>/.wisp/.staging/<rand>/ in production) on the SAME filesystem as
// <root>/.wisp/modules/; each fetched module's tree is staged into
// <stagingRoot>/<owner>/<repo> so Task 8 can place it with a same-FS os.Rename (no
// EXDEV). root is the root wisp.json dependencies. oldLock (may be nil) lets a caller
// mark commit/url-identical modules for reuse (Task 8); when nil everything resolves
// fresh. It returns the resolved reachable module set keyed by owner/repo, or a hard
// error on a transitive conflict or any validation/fetch failure.
func resolveGraph(f Fetcher, stagingRoot string, root map[string]dependency, oldLock map[string]lockEntry) (map[string]resolved, error) {
	out := map[string]resolved{}
	// pins[key] is the winning pin. Root pins are PRE-SEEDED below so a root override
	// is order-independent: any transitive pin for a key already pinned-as-root is a
	// no-op regardless of when it is encountered.
	pins := map[string]pin{}

	// queue of (key, dependency, parent, isRoot)
	type item struct {
		key    string
		dep    dependency
		parent string
		isRoot bool
	}

	// resolvePin validates a dependency and resolves its ref to a commit.
	resolvePin := func(key string, d dependency, parent string, isRoot bool) (pin, error) {
		if _, _, err := module.SplitRepo(key); err != nil {
			return pin{}, fmt.Errorf("dependency key %q (from %s): %v", key, parent, err)
		}
		if err := validateGitURL(d.Git); err != nil {
			return pin{}, fmt.Errorf("dependency %q (from %s): %v", key, parent, err)
		}
		if err := validateRef(d.Ref); err != nil {
			return pin{}, fmt.Errorf("dependency %q (from %s): %v", key, parent, err)
		}
		commit := d.Ref
		if !isFullSHA(d.Ref) {
			c, err := f.Resolve(d.Git, d.Ref)
			if err != nil {
				return pin{}, fmt.Errorf("%s (%s): resolve ref %q: %v", key, d.Git, d.Ref, err)
			}
			commit = c
		}
		if err := validateCommit(commit); err != nil {
			return pin{}, fmt.Errorf("%s (%s): %v", key, d.Git, err)
		}
		return pin{commit: commit, url: d.Git, ref: d.Ref, parent: parent, isRoot: isRoot}, nil
	}

	// PRE-SEED: resolve every root key, write it into pins as a root pin, and seed the
	// queue from the root keys (sorted for deterministic walk order). Doing this before
	// the walk makes root override independent of the order transitive pins arrive in.
	rootKeys := sortedKeys(root)
	var queue []item
	for _, k := range rootKeys {
		p, err := resolvePin(k, root[k], "<root>", true)
		if err != nil {
			return nil, err
		}
		pins[k] = p
		queue = append(queue, item{k, root[k], "<root>", true})
	}

	visited := map[string]bool{}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]

		var p pin
		if it.isRoot {
			// Root items are pre-seeded in pins; walk under the root pin.
			p = pins[it.key]
		} else {
			// Transitive item: BEFORE resolving (querying the fetcher), check whether this
			// key is already pinned-as-root. If so, the root override wins and the
			// transitive version is NEVER queried/resolved or walked -- matching the
			// "root override never queries/walks the transitive version" prose. Doing the
			// isRoot check FIRST avoids a redundant f.Resolve call for the transitive ref.
			if existing, had := pins[it.key]; had && existing.isRoot {
				continue
			}
			// Not root-pinned: resolve its pin and reconcile against any existing pin.
			np, err := resolvePin(it.key, it.dep, it.parent, it.isRoot)
			if err != nil {
				return nil, err
			}
			if existing, had := pins[it.key]; had {
				if existing.commit != np.commit || existing.url != np.url {
					return nil, fmt.Errorf(
						"transitive conflict for %q: %s wants commit %s url %s, but %s wants commit %s url %s",
						it.key, existing.parent, existing.commit, existing.url,
						np.parent, np.commit, np.url)
				}
				// Same transitive pin as already recorded; fall through to the
				// visited check, which skips a re-walk.
			} else {
				pins[it.key] = np
			}
			p = pins[it.key]
		}

		if visited[it.key] {
			continue
		}
		visited[it.key] = true

		// Check lock reuse: if oldLock has an entry for this key whose commit+git match
		// the selecting pin, reuse the cached graph data (deps + treeDigest) and skip
		// the fetch+manifest-read. The reuse entry's identity fields (git/ref/commit)
		// always come from the CURRENT selecting pin, never from oldLock.
		if oldLock != nil {
			if ole, ok := oldLock[it.key]; ok && ole.Commit == p.commit && ole.Git == p.url {
				deps := ole.Deps
				if deps == nil {
					deps = []string{}
				}
				out[it.key] = resolved{
					git: p.url, ref: p.ref, commit: p.commit,
					treeDigest: ole.TreeDigest,
					deps:       deps,
					dir:        "", // reuse: tree not fetched fresh; caller verifies on-disk
				}
				// Enqueue children using each child's OWN lock entry as the dependency
				// source (INVARIANT: readLock rejects dangling dep edges, so every child
				// key here has its own oldLock entry).
				for _, ck := range deps {
					cole, ok := oldLock[ck]
					if !ok {
						return nil, fmt.Errorf("internal: reused module %q dep %q has no lock entry (corrupt lock after readLock)", it.key, ck)
					}
					queue = append(queue, item{ck, dependency{Git: cole.Git, Ref: cole.Ref}, it.key, false})
				}
				continue
			}
		}

		// Fetch fresh to read its own dependencies. Stage into <stagingRoot>/<owner>/<repo>
		// so the tree lands on the same filesystem as <root>/.wisp/modules/ and Task 8's
		// os.Rename never hits EXDEV. it.key is already SplitRepo-validated (resolvePin),
		// so it cannot escape.
		dir := filepath.Join(stagingRoot, filepath.FromSlash(it.key))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := f.Fetch(p.url, p.ref, p.commit, dir); err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("%s (%s): fetch commit %s: %v", it.key, p.url, p.commit, err)
		}
		childMan, err := readManifest(filepath.Join(dir, "wisp.json"))
		if err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("%s: %v", it.key, err)
		}
		depKeys := sortedKeys(childMan.Dependencies)
		out[it.key] = resolved{
			git: p.url, ref: p.ref, commit: p.commit,
			deps: depKeys, dir: dir,
		}
		for _, dk := range depKeys {
			queue = append(queue, item{dk, childMan.Dependencies[dk], it.key, false})
		}
	}

	// Prune to reachable set: BFS from root keys over recorded deps. (Everything in
	// `out` was reached by construction, but a root-override may have left a stale
	// transitive entry that is no longer reachable; recompute to be safe.)
	reachable := reachableSet(rootKeys, out)
	for k := range out {
		if !reachable[k] {
			os.RemoveAll(out[k].dir)
			delete(out, k)
		}
	}
	return out, nil
}

func reachableSet(rootKeys []string, mods map[string]resolved) map[string]bool {
	seen := map[string]bool{}
	var stack []string
	stack = append(stack, rootKeys...)
	for len(stack) > 0 {
		k := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[k] {
			continue
		}
		seen[k] = true
		if r, ok := mods[k]; ok {
			stack = append(stack, r.deps...)
		}
	}
	return seen
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
