package pkgtool

// Fetcher is the seam at the fetch boundary (spec section 9). Resolve maps a
// symbolic ref to an immutable commit SHA; Fetch obtains the pinned commit and
// materializes its tree (regular files + dirs only, no .git) into destDir. Fetch
// receives ref AS WELL AS sha because the section-5 fallback for a SHA-refusing
// host needs the ref tip. DefaultRef discovers the remote's default branch NAME
// (e.g. "main") so a no-ref first `add` records a concrete branch ref, never the
// literal "HEAD".
type Fetcher interface {
	DefaultRef(url string) (refName string, err error)
	Resolve(url, ref string) (sha string, err error)
	Fetch(url, ref, sha, destDir string) error
}
