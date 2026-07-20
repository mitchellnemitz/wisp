package pkgtool

import "testing"

func TestValidateGitURL(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/o/r.git", false},
		{"ssh://git@example.com/o/r.git", false},
		{"git://example.com/o/r.git", false},
		{"file:///tmp/repo.git", false},
		{"git@github.com:o/r.git", false},        // scp-like
		{"git@github.com:o/r::weird.git", false}, // scp-like with :: in the PATH is OK
		{"ext::sh -c touch /tmp/pwned", true},
		{"fd::17", true},
		{"transport::x", true},
		{"-oProxyCommand=evil", true}, // leading dash
		{"", true},
		{"https://example.com\nevil", true}, // embedded newline
		{"https://example.com\tevil", true}, // embedded tab (control char)
		{"https://example.com\x1bevil", true},
		{"https://example.com\x7frepo", true},
		// ssh option-injection: a host starting with '-' is passed to ssh as an argv
		// element, so '--' on the git command does not protect it.
		{"ssh://-oProxyCommand=evil/repo.git", true},
		{"ssh://git@-oProxyCommand=evil/repo.git", true},
		{"-oProxyCommand=evil@host:repo.git", true}, // leading-dash caught early
		{"git@-oProxyCommand=evil:repo.git", true},  // scp-like host starting with '-'
	}
	for _, c := range cases {
		if err := validateGitURL(c.url); (err != nil) != c.wantErr {
			t.Errorf("validateGitURL(%q) err=%v wantErr=%v", c.url, err, c.wantErr)
		}
	}
}

func TestValidateRef(t *testing.T) {
	cases := []struct {
		ref     string
		wantErr bool
	}{
		{"v1.2.0", false},
		{"main", false},
		{"feature/x", false},
		{"refs/tags/v1", false},
		{"-rf", true},      // leading dash (git-flag injection)
		{"a;rm -rf", true}, // shell metachar
		{"a b", true},      // space
		{"", true},
		{"a$(x)", true},
	}
	for _, c := range cases {
		if err := validateRef(c.ref); (err != nil) != c.wantErr {
			t.Errorf("validateRef(%q) err=%v wantErr=%v", c.ref, err, c.wantErr)
		}
	}
}

func TestIsFullSHAAndValidateCommit(t *testing.T) {
	if !isFullSHA(hex40("a")) {
		t.Error("40-hex should be a full SHA")
	}
	if !isFullSHA(hex64("a")) {
		t.Error("64-hex should be a full SHA")
	}
	if isFullSHA("abc1234") {
		t.Error("abbreviated should not be a full SHA")
	}
	if err := validateCommit(hex40("a")); err != nil {
		t.Errorf("validateCommit(40-hex) = %v", err)
	}
	if err := validateCommit("abc1234"); err == nil {
		t.Error("abbreviated commit must be rejected in the lock")
	}
}
