package pkgtool

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// refRe forbids a leading '-' (git-flag injection) and shell metacharacters.
	// Leading char is a letter or digit; the body allows / . _ - for ref names.
	refRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	sha1Re   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// treeDigestRe is the canonical lock treeDigest form: "sha256:" + 64 lowercase hex
	// (the output shape of treeDigest()).
	treeDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	// scpLikeRe matches the scp-like form user@host:path (no scheme).
	scpLikeRe = regexp.MustCompile(`^[A-Za-z0-9._~-]+@[A-Za-z0-9._-]+:[^\x00-\x1f]*$`)
)

func isFullSHA(s string) bool { return sha1Re.MatchString(s) || sha256Re.MatchString(s) }

func validateCommit(s string) error {
	if !isFullSHA(s) {
		return fmt.Errorf("commit %q is not a full 40/64-hex SHA", s)
	}
	return nil
}

func validateRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("empty ref")
	}
	if !refRe.MatchString(ref) {
		return fmt.Errorf("ref %q contains an unsafe character or a leading dash", ref)
	}
	return nil
}

// validateTreeDigest enforces the canonical lock digest form "sha256:" + 64 lowercase hex
// (the shape treeDigest emits). An empty or malformed digest is a corrupt-lock error.
func validateTreeDigest(d string) error {
	if !treeDigestRe.MatchString(d) {
		return fmt.Errorf("treeDigest %q is not a valid sha256:<64-hex> digest", d)
	}
	return nil
}

// validateGitURL enforces the transport allowlist (spec section 7). Allowed:
// https://, ssh://, git://, file://, and the scp-like user@host:path form.
// Rejected: ext::, fd:: and any other transport-helper, a leading '-', control
// characters, and the empty string. Applies to both --git and transitive URLs.
func validateGitURL(url string) error {
	if url == "" {
		return fmt.Errorf("empty git url")
	}
	if strings.HasPrefix(url, "-") {
		return fmt.Errorf("git url %q must not start with '-'", url)
	}
	// Reject ALL ASCII control characters (< 0x20) and DEL (0x7f), not just NUL/LF/CR:
	// a tab/ESC in a URL is never legitimate and enables log/terminal injection.
	for i := 0; i < len(url); i++ {
		if url[i] < 0x20 || url[i] == 0x7f {
			return fmt.Errorf("git url %q contains a control character", url)
		}
	}
	// Match an allowlisted scheme first. For ssh:// the host is passed to ssh as an
	// argv element, so a host starting with '-' is an ssh option-injection (e.g.
	// ssh://-oProxyCommand=.../repo) -- reject it (the git-level '--' does not protect
	// the ssh argv).
	for _, scheme := range []string{"https://", "ssh://", "git://", "file://"} {
		if strings.HasPrefix(url, scheme) {
			if scheme == "ssh://" {
				rest := strings.TrimPrefix(url, scheme)
				if at := strings.IndexByte(rest, '@'); at >= 0 {
					rest = rest[at+1:] // strip optional user@
				}
				host := rest
				if slash := strings.IndexByte(host, '/'); slash >= 0 {
					host = host[:slash]
				}
				if strings.HasPrefix(host, "-") {
					return fmt.Errorf("git url %q has an ssh host starting with '-' (option injection)", url)
				}
			}
			return nil
		}
	}
	// Then the scp-like user@host:path form (which may legitimately contain "::" in the
	// path component). Matching it before the transport-helper check avoids rejecting a
	// valid scp URL whose path happens to contain "::". The host (between '@' and ':')
	// is passed to ssh as an argv element, so reject a host starting with '-'.
	if scpLikeRe.MatchString(url) {
		at := strings.IndexByte(url, '@')
		colon := strings.IndexByte(url, ':')
		host := url[at+1 : colon]
		if strings.HasPrefix(host, "-") {
			return fmt.Errorf("git url %q has an scp-like host starting with '-' (option injection)", url)
		}
		return nil
	}
	// A git transport-helper is "<helper>::..." e.g. ext::, fd::, transport::. The "::"
	// must be scoped to the SCHEME PREFIX -- the text before the first '/' (and before
	// any '@'); a blunt strings.Contains(url, "::") would also reject a legitimate scp
	// path that contains "::". Only reject when "::" appears in that leading prefix.
	prefix := url
	if i := strings.IndexAny(url, "/@"); i >= 0 {
		prefix = url[:i]
	}
	if strings.Contains(prefix, "::") {
		return fmt.Errorf("git url %q uses a disallowed transport helper", url)
	}
	return fmt.Errorf("git url %q is not an allowed transport (https/ssh/git/file/scp-like)", url)
}
