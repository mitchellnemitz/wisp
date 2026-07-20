package pkgtool

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/version"
)

// applyMutation describes the kind of live-tree change that was just committed.
// Tests inject failures via applyHook to exercise backup/restore paths.
type applyMutation int

const (
	applyPlace     applyMutation = iota // a brand-new module dir was placed
	applyOverwrite                      // an existing module dir was replaced in-place
	applyPrune                          // an unreachable module dir was removed
)

// applyHook is called after each live-dir mutation with the kind of change.
// Nil in production. Tests override it to inject mid-apply failures.
var applyHook func(applyMutation) error

// installAt resolves the dependency graph from wisp.json, materialises the
// reachable set into <root>/.wisp/modules/, and writes wisp.lock. It is the
// implementation of `wisp install`.
//
// Consistency rule (spec 4.3): when the existing wisp.lock is "consistent"
// (every root manifest dep's git+ref byte-equal to its lock entry) the lock is
// trusted pin-wins -- no remote queries, no Resolve/DefaultRef calls. Any
// reachable module whose on-disk tree is absent is re-fetched using the locked
// commit; any module whose digest disagrees with the lock is a hard error.
//
// Atomic placement: every live-dir mutation is preceded by a backup into
// <root>/.wisp/.backup/<rand>/. On any error the backup is re-played and the
// staging dir removed. If .wisp/ did not exist before the command, the entire
// .wisp/ dir is removed on failure. .wisp/.gitignore is written LAST on success.
func installAt(root string, f Fetcher, warnw io.Writer) error {
	wispDir := filepath.Join(root, ".wisp")
	// Refuse a symlinked (or non-dir) .wisp AND the subdirectories we write through
	// (modules, .staging, .backup): os.Stat/MkdirAll/Rename follow symlinks, so a
	// malicious checkout shipping any of these as a symlink could redirect writes
	// outside the project root. Check before any write touches them.
	if err := refuseSymlinkedDir(wispDir); err != nil {
		return err
	}
	for _, sub := range []string{"modules", ".staging", ".backup"} {
		if err := refuseSymlinkedDir(filepath.Join(wispDir, sub)); err != nil {
			return err
		}
	}
	wispExisted := dirExists(wispDir)

	man, err := readManifest(filepath.Join(root, "wisp.json"))
	if err != nil {
		return err
	}

	lockPath := filepath.Join(root, "wisp.lock")
	var oldLock *lockfile
	if lf, err := readLock(lockPath); err == nil {
		oldLock = lf
		warnIfCompilerMismatch(warnw, lf.Wisp, version.Number)
	} else if !errors.Is(err, os.ErrNotExist) {
		// A present-but-corrupt lock (bad version/commit/digest, dangling edge,
		// malformed JSON) is a hard error -- never silently re-resolve past it,
		// which would hide tampering/corruption the lock integrity check detects.
		return fmt.Errorf("wisp.lock: %w", err)
	}

	// Determine whether the existing lock is consistent with the current manifest.
	// The lock is consistent when:
	//   (a) every manifest direct dep has a lock entry with byte-equal git+ref, AND
	//   (b) the lock's reachable set (BFS from manifest root keys using lock edges)
	//       covers exactly all keys in the lock -- no lock entry is orphaned.
	// Condition (b) ensures that dropping a direct dep marks the lock stale even if
	// the remaining entries all match (the orphaned entries would never be pruned by
	// the consistent path, which does not re-resolve). With NO existing lock,
	// consistent stays false and installAt takes the stale/first-time path (which
	// writes a fresh lock) regardless of dep count -- including a zero-dep manifest,
	// which writes an empty lock. A zero-dep manifest WITH a lock that has entries is
	// stale (orphaned entries to prune).
	consistent := false
	if oldLock != nil {
		if len(man.Dependencies) == 0 && len(oldLock.Modules) == 0 {
			consistent = true
		} else if len(man.Dependencies) > 0 {
			consistent = true
			// (a) every manifest dep matched by lock entry.
			for key, d := range man.Dependencies {
				le, ok := oldLock.Modules[key]
				if !ok || le.Git != d.Git || le.Ref != d.Ref {
					consistent = false
					break
				}
			}
			// (b) lock's reachable set from manifest root keys must equal the full lock.
			if consistent {
				rootKeys := sortedKeys(man.Dependencies)
				// Build a pseudo-resolved map from the lock so reachableSet can walk edges.
				pseudoMods := make(map[string]resolved, len(oldLock.Modules))
				for k, le := range oldLock.Modules {
					pseudoMods[k] = resolved{deps: le.Deps}
				}
				reachable := reachableSet(rootKeys, pseudoMods)
				if len(reachable) != len(oldLock.Modules) {
					consistent = false
				}
			}
		}
	}

	// Build a random staging dir and backup dir on the same filesystem as
	// .wisp/modules/ so os.Rename never crosses a device boundary.
	if err := os.MkdirAll(wispDir, 0o755); err != nil {
		return err
	}
	stagingID, err := randHex()
	if err != nil {
		cleanupOnFail(wispDir, wispExisted)
		return err
	}
	backupID, err := randHex()
	if err != nil {
		cleanupOnFail(wispDir, wispExisted)
		return err
	}
	stagingRoot := filepath.Join(wispDir, ".staging", stagingID)
	backupRoot := filepath.Join(wispDir, ".backup", backupID)
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		cleanupOnFail(wispDir, wispExisted)
		return err
	}
	if err := os.MkdirAll(backupRoot, 0o755); err != nil {
		os.RemoveAll(stagingRoot)
		cleanupOnFail(wispDir, wispExisted)
		return err
	}
	defer os.RemoveAll(stagingRoot)
	defer os.RemoveAll(backupRoot)

	modulesDir := filepath.Join(wispDir, "modules")

	// ---------- CONSISTENT PATH ----------
	if consistent {
		return installConsistent(root, wispDir, modulesDir, stagingRoot, backupRoot, f, oldLock, lockPath, wispExisted)
	}

	// ---------- STALE / FIRST-TIME PATH ----------
	return installStale(root, wispDir, modulesDir, stagingRoot, backupRoot, f, man, oldLock, lockPath, wispExisted)
}

// installConsistent handles the spec 4.3 "pin-wins" path: trust the lock,
// rebuild only absent/mismatched trees, hard-error on tampered trees.
func installConsistent(
	root, wispDir, modulesDir, stagingRoot, backupRoot string,
	f Fetcher,
	lf *lockfile,
	lockPath string,
	wispExisted bool,
) (retErr error) {
	// On ANY error (including the verify/fetch loop's early returns below, not just
	// apply-phase failures), remove a .wisp/ that did not exist before this command,
	// so a failed consistent install on a clean checkout leaves the project unchanged.
	defer func() {
		if retErr != nil {
			cleanupOnFail(wispDir, wispExisted)
		}
	}()

	// The lock encodes the full reachable set by construction (no remote queries on
	// the consistent path): every locked key is reachable, and any on-disk module dir
	// NOT in the lock is cruft to prune so the tree is reproduced byte-identically.
	allKeys := sortedKeys(lf.Modules)
	reachable := make(map[string]bool, len(allKeys))
	for _, k := range allKeys {
		reachable[k] = true
	}

	// staged maps key -> staged dir (for newly-fetched modules).
	staged := map[string]string{}
	for _, key := range allKeys {
		le := lf.Modules[key]
		// Refuse a symlinked <owner> / <owner>/<repo> before reading or writing
		// through it; this guard covers every later MkdirAll/Rename for this key
		// (apply runs single-threaded after this loop).
		if err := refuseSymlinkedModulePath(modulesDir, key); err != nil {
			return err
		}
		modDir := filepath.Join(modulesDir, filepath.FromSlash(key))

		if dirExists(modDir) {
			// Tree present: verify digest.
			got, err := treeDigest(modDir)
			if err != nil {
				return fmt.Errorf("%s: recompute digest: %v", key, err)
			}
			if got != le.TreeDigest {
				return fmt.Errorf("%s: on-disk tree digest %s does not match lock %s (tampering or partial write)", key, got, le.TreeDigest)
			}
			// Digest matches: nothing to do for this module.
			staged[key] = "" // empty = already in place
			continue
		}

		// Tree absent: fetch into staging.
		dir := filepath.Join(stagingRoot, filepath.FromSlash(key))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := f.Fetch(le.Git, le.Ref, le.Commit, dir); err != nil {
			return fmt.Errorf("%s: fetch commit %s: %v", key, le.Commit, err)
		}
		// Verify digest matches lock.
		got, err := treeDigest(dir)
		if err != nil {
			return fmt.Errorf("%s: compute staged digest: %v", key, err)
		}
		if got != le.TreeDigest {
			return fmt.Errorf("%s: fetched tree digest %s does not match lock %s", key, got, le.TreeDigest)
		}
		staged[key] = dir
	}

	// Apply phase: all live-dir mutations go through the backup/restore mechanism.
	// (The deferred cleanup above handles the .wisp/ removal; applyFail only rolls back.)
	var backups []backup
	applyFail := func(err error) error {
		if rbErr := rollback(backups); rbErr != nil {
			return fmt.Errorf("%w; ALSO rollback incomplete, project may not be restored: %v", err, rbErr)
		}
		return err
	}

	// First: prune any on-disk module dir not in the lock (unreachable cruft), so a
	// consistent install reproduces a byte-identical .wisp/modules tree (AC4).
	if err := pruneUnreachable(modulesDir, backupRoot, reachable, &backups); err != nil {
		return applyFail(err)
	}

	// Second: place each absent (newly-fetched) module from staging.
	for _, key := range allKeys {
		dir, ok := staged[key]
		if !ok || dir == "" {
			continue // already present and verified
		}
		modDir := filepath.Join(modulesDir, filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(modDir), 0o755); err != nil {
			return applyFail(err)
		}
		// modDir does not exist (we only got here because dirExists was false).
		if err := os.Rename(dir, modDir); err != nil {
			return applyFail(err)
		}
		backups = append(backups, backup{modDir: modDir, backupDir: ""}) // new placement; revert = remove
		if applyHook != nil {
			if err := applyHook(applyPlace); err != nil {
				return applyFail(err)
			}
		}
	}

	removeEmptyOwnerDirs(modulesDir)
	writeGitignore(wispDir)
	return nil
}

// installStale handles the first-time or stale path: resolves the graph,
// computes digests, and atomically updates the module tree. It is a thin driver
// over three focused steps -- resolve+verify (verifyOrRefetch), lock
// construction (buildLockfile), and the two-phase apply (applyStaleModules).
//
// Cleanup policy is centralized in the single deferred cleanupOnFail below (the
// same shape installConsistent uses): ANY error return -- pre-apply or
// apply-phase -- removes a .wisp/ that did not exist before the command. The
// apply phase additionally rolls back its live-dir mutations via applyFail
// BEFORE that cleanup runs, so an existing .wisp/ is restored in place.
func installStale(
	root, wispDir, modulesDir, stagingRoot, backupRoot string,
	f Fetcher,
	man *manifest,
	oldLock *lockfile,
	lockPath string,
	wispExisted bool,
) (retErr error) {
	defer func() {
		if retErr != nil {
			cleanupOnFail(wispDir, wispExisted)
		}
	}()

	var oldLockMap map[string]lockEntry
	if oldLock != nil {
		oldLockMap = oldLock.Modules
	}

	mods, err := resolveGraph(f, stagingRoot, man.Dependencies, oldLockMap)
	if err != nil {
		return err
	}

	keys := sortedKeys(mods)
	if err := verifyOrRefetch(f, modulesDir, stagingRoot, mods, keys); err != nil {
		return err
	}

	// Stage the new lock to disk (temp bytes for the apply phase's atomic write).
	lockBytes, err := marshalLock(buildLockfile(mods, keys))
	if err != nil {
		return err
	}

	// Determine the current reachable set (which module dirs should survive).
	rootKeys := sortedKeys(man.Dependencies)
	reachableMap := reachableSet(rootKeys, mods)

	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		return err
	}

	return applyStaleModules(wispDir, modulesDir, backupRoot, lockPath, lockBytes, mods, keys, reachableMap)
}

// verifyOrRefetch resolves the digest of every module in mods:
//   - reused (dir==""), tree present: recompute and verify against the lock digest.
//   - reused (dir==""), tree absent: re-fetch into <stagingRoot>/refetch/<key>,
//     verify, and record the staged dir for the apply phase to place.
//   - freshly fetched (dir!=""): compute and record the tree digest.
//
// It mutates mods in place (setting r.dir/r.treeDigest) and returns any error
// unwrapped for the caller's deferred cleanup to handle. The symlink guard runs
// for every key here so the later apply MkdirAll/Rename is covered too.
func verifyOrRefetch(f Fetcher, modulesDir, stagingRoot string, mods map[string]resolved, keys []string) error {
	for _, key := range keys {
		r := mods[key]
		if err := refuseSymlinkedModulePath(modulesDir, key); err != nil {
			return err
		}
		if r.dir == "" {
			// Reused from lock: the on-disk tree should already be there.
			// If absent, we need to re-fetch it.
			modDir := filepath.Join(modulesDir, filepath.FromSlash(key))
			if !dirExists(modDir) {
				// Re-fetch into staging.
				dir := filepath.Join(stagingRoot, "refetch", filepath.FromSlash(key))
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				if err := f.Fetch(r.git, r.ref, r.commit, dir); err != nil {
					return fmt.Errorf("%s (%s): re-fetch commit %s: %v", key, r.git, r.commit, err)
				}
				got, err := treeDigest(dir)
				if err != nil {
					return fmt.Errorf("%s: compute digest: %v", key, err)
				}
				// The re-fetched tree MUST match the reused lock digest; otherwise a
				// tampered lock digest would be silently "fixed up" rather than
				// reported (mirrors the consistent/present-reused paths).
				if got != r.treeDigest {
					return fmt.Errorf("%s: re-fetched tree digest %s does not match lock %s (tampered lock?)", key, got, r.treeDigest)
				}
				r.dir = dir
				mods[key] = r
			} else {
				// Present: verify the on-disk tree against the reused lock digest
				// (tamper detection), same as the consistent path -- do not blindly
				// trust an existing reused tree.
				got, err := treeDigest(modDir)
				if err != nil {
					return fmt.Errorf("%s: recompute digest: %v", key, err)
				}
				if got != r.treeDigest {
					return fmt.Errorf("%s: on-disk tree digest %s does not match lock %s (tampering or partial write)", key, got, r.treeDigest)
				}
			}
		} else {
			// Freshly fetched: compute treeDigest from staged dir.
			digest, err := treeDigest(r.dir)
			if err != nil {
				return fmt.Errorf("%s: compute digest: %v", key, err)
			}
			r.treeDigest = digest
			mods[key] = r
		}
	}
	return nil
}

// buildLockfile assembles the new lockfile from the resolved-and-verified module
// set. keys must be sorted; each entry's Deps is copied and sorted so the
// serialized lock is deterministic.
func buildLockfile(mods map[string]resolved, keys []string) *lockfile {
	newLF := &lockfile{Version: lockVersion, Wisp: version.Number, Modules: map[string]lockEntry{}}
	for _, key := range keys {
		r := mods[key]
		depsCopy := append([]string{}, r.deps...)
		sort.Strings(depsCopy)
		newLF.Modules[key] = lockEntry{
			Git:        r.git,
			Ref:        r.ref,
			Commit:     r.commit,
			TreeDigest: r.treeDigest,
			Deps:       depsCopy,
		}
	}
	return newLF
}

// warnIfCompilerMismatch emits one advisory line when an existing lock was
// written by a different, known compiler. Both the dev sentinel and an
// absent recorded version are treated as "don't warn".
func warnIfCompilerMismatch(w io.Writer, recorded, running string) {
	const dev = "0.0.0-dev"
	if recorded == "" || running == "" || recorded == dev || running == dev {
		return
	}
	if recorded != running {
		fmt.Fprintf(w, "wisp: warning: wisp.lock was written by wisp %s, but this is wisp %s\n", recorded, running)
	}
}

// applyStaleModules is the stale path's two-phase apply: prune unreachable dirs,
// then place/overwrite reachable modules, then write the lock -- every live-dir
// mutation routed through the backup/restore mechanism. On any error it rolls
// back the recorded backups (applyFail); the caller's deferred cleanupOnFail
// then removes a .wisp/ that did not pre-exist. On success it tidies empty owner
// dirs and writes .gitignore LAST.
func applyStaleModules(
	wispDir, modulesDir, backupRoot, lockPath string,
	lockBytes []byte,
	mods map[string]resolved,
	keys []string,
	reachable map[string]bool,
) error {
	var backups []backup

	applyFail := func(err error) error {
		if rbErr := rollback(backups); rbErr != nil {
			return fmt.Errorf("%w; ALSO rollback incomplete, project may not be restored: %v", err, rbErr)
		}
		return err
	}

	// First: prune unreachable module dirs that exist on disk.
	if err := pruneUnreachable(modulesDir, backupRoot, reachable, &backups); err != nil {
		return applyFail(err)
	}

	// Second: place or overwrite reachable modules.
	for _, key := range keys {
		r := mods[key]
		modDir := filepath.Join(modulesDir, filepath.FromSlash(key))

		if r.dir == "" {
			// Reused entry with an on-disk tree that already matched.
			continue
		}

		if err := os.MkdirAll(filepath.Dir(modDir), 0o755); err != nil {
			return applyFail(err)
		}

		kind := applyPlace
		if dirExists(modDir) {
			// Overwrite: move existing aside to backup first.
			bkDir := filepath.Join(backupRoot, "overwrite", filepath.FromSlash(key))
			if err := os.MkdirAll(filepath.Dir(bkDir), 0o755); err != nil {
				return applyFail(err)
			}
			if err := os.Rename(modDir, bkDir); err != nil {
				return applyFail(err)
			}
			backups = append(backups, backup{modDir: modDir, backupDir: bkDir})
			kind = applyOverwrite
		}

		if err := os.Rename(r.dir, modDir); err != nil {
			return applyFail(err)
		}
		if kind == applyPlace {
			// Record as new placement so rollback knows to remove it.
			backups = append(backups, backup{modDir: modDir, backupDir: ""})
		}
		if applyHook != nil {
			if err := applyHook(kind); err != nil {
				return applyFail(err)
			}
		}
	}

	// Write the lock file.
	if err := writeFileAtomic(lockPath, lockBytes); err != nil {
		return applyFail(err)
	}

	removeEmptyOwnerDirs(modulesDir)
	writeGitignore(wispDir)
	return nil
}

// pruneUnreachable moves aside every on-disk module dir under modulesDir whose
// key is not in reachable, recording each move in *backups so a rollback can
// restore it. It is the shared apply-phase prune step for both install paths
// (consistent and stale). Errors are returned unwrapped: the caller feeds them
// through its own applyFail so rollback and .wisp/ cleanup run with that path's
// backup set and cleanup policy. Each backup is appended BEFORE applyHook fires,
// so a hook failure still rolls the just-moved dir back.
func pruneUnreachable(modulesDir, backupRoot string, reachable map[string]bool, backups *[]backup) error {
	if !dirExists(modulesDir) {
		return nil
	}
	existing, err := listModuleKeys(modulesDir)
	if err != nil {
		return err
	}
	for _, key := range existing {
		if reachable[key] {
			continue
		}
		modDir := filepath.Join(modulesDir, filepath.FromSlash(key))
		bkDir := filepath.Join(backupRoot, "prune", filepath.FromSlash(key))
		if err := os.MkdirAll(filepath.Dir(bkDir), 0o755); err != nil {
			return err
		}
		if err := os.Rename(modDir, bkDir); err != nil {
			return err
		}
		*backups = append(*backups, backup{modDir: modDir, backupDir: bkDir, pruned: true})
		if applyHook != nil {
			if err := applyHook(applyPrune); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- backup/restore helpers ---

type backup struct {
	modDir    string // live location
	backupDir string // backup location ("" means it was newly placed, so rollback = remove)
	pruned    bool   // was moved aside as a prune (rollback = restore)
}

// rollback replays backups in reverse order. It returns a non-nil error if any
// restore step fails, so the caller can report that the project may NOT be
// byte-identical to its pre-command state despite the atomicity contract.
func rollback(bs []backup) error {
	var errs []string
	for i := len(bs) - 1; i >= 0; i-- {
		b := bs[i]
		if b.backupDir == "" {
			// Newly placed: remove.
			if err := os.RemoveAll(b.modDir); err != nil {
				errs = append(errs, fmt.Sprintf("remove %q: %v", b.modDir, err))
			}
		} else {
			// Overwritten or pruned: restore.
			if err := os.RemoveAll(b.modDir); err != nil { // remove whatever is there now
				errs = append(errs, fmt.Sprintf("clear %q: %v", b.modDir, err))
				continue
			}
			if err := os.Rename(b.backupDir, b.modDir); err != nil {
				errs = append(errs, fmt.Sprintf("restore %q: %v", b.modDir, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// cleanupOnFail removes .wisp/ entirely when it did not exist before the command.
func cleanupOnFail(wispDir string, wispExisted bool) {
	if !wispExisted {
		os.RemoveAll(wispDir)
	}
}

// listModuleKeys returns all "owner/repo" keys that exist as physical dirs
// under modulesDir (two-level walk: owner/repo).
func listModuleKeys(modulesDir string) ([]string, error) {
	owners, err := os.ReadDir(modulesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var keys []string
	for _, o := range owners {
		// A symlinked owner entry is a hard error: os.ReadDir/Rename below would
		// follow it out of the project tree. Never silently skip it.
		if o.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("module owner %q is a symlink; refusing to operate", o.Name())
		}
		if !o.IsDir() {
			continue
		}
		repos, err := os.ReadDir(filepath.Join(modulesDir, o.Name()))
		if err != nil {
			// An unreadable owner dir would make us miss its module(s) and skip a
			// required prune, silently breaking byte-identical reproduction. Fail.
			return nil, fmt.Errorf("read module owner dir %q: %v", o.Name(), err)
		}
		for _, r := range repos {
			if r.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("module %q/%q is a symlink; refusing to operate", o.Name(), r.Name())
			}
			if r.IsDir() {
				keys = append(keys, o.Name()+"/"+r.Name())
			}
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// removeEmptyOwnerDirs removes any <owner> directory under modulesDir that has no
// remaining <repo> children, so a prune of <owner>/<repo> does not leave an empty
// owner dir behind. Best-effort, called on success after all placements/prunes.
func removeEmptyOwnerDirs(modulesDir string) {
	owners, err := os.ReadDir(modulesDir)
	if err != nil {
		return
	}
	for _, o := range owners {
		if !o.IsDir() {
			continue
		}
		ownerPath := filepath.Join(modulesDir, o.Name())
		ents, err := os.ReadDir(ownerPath)
		if err == nil && len(ents) == 0 {
			os.Remove(ownerPath)
		}
	}
}

// refuseSymlinkedDir errors if path exists and is a symlink or a non-directory.
// Used to refuse following an attacker-planted symlink (or a stray file) for the
// .wisp/ tree the package tool writes through, which could redirect writes
// outside the project root. A nonexistent path is fine (it will be created).
// Only os.ErrNotExist is treated as "absent"; any other Lstat failure (e.g. a
// permission error on an existing path) is surfaced, so an uninspectable path
// can never silently pass the "no symlinks before we write" guard.
func refuseSymlinkedDir(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // does not exist -> created fresh by MkdirAll
		}
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to operate (it could redirect writes outside the project)", path)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s exists and is not a directory; refusing to operate", path)
	}
	return nil
}

// refuseSymlinkedModulePath rejects a symlink (or stray non-dir) at the <owner>
// or <owner>/<repo> path components for a module key under modulesDir, before
// any digest read, MkdirAll, or Rename targets that path. os.Stat/MkdirAll/
// Rename follow symlinks, so a pre-existing symlinked component (e.g. a
// committed .wisp/modules/owner -> /tmp) would otherwise redirect reads/writes
// outside the project root, bypassing the top-level .wisp/modules guard.
func refuseSymlinkedModulePath(modulesDir, key string) error {
	owner, repo, _ := strings.Cut(key, "/")
	comps := []string{owner}
	if repo != "" {
		comps = append(comps, filepath.Join(owner, repo))
	}
	for _, c := range comps {
		if err := refuseSymlinkedDir(filepath.Join(modulesDir, filepath.FromSlash(c))); err != nil {
			return err
		}
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func randHex() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// writeGitignore writes .wisp/.gitignore so that modules/ is ignored.
// Called only on success, LAST. Failures here are silently swallowed (best-effort).
func writeGitignore(wispDir string) {
	content := "modules/\n.staging/\n.backup/\n"
	path := filepath.Join(wispDir, ".gitignore")
	_ = writeFileAtomic(path, []byte(content))
}
