package core

import "testing"

func TestNamespacesContainsJSON(t *testing.T) {
	found := false
	for _, ns := range Namespaces() {
		if ns == "json" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Namespaces() = %v, want it to contain \"json\"", Namespaces())
	}
}

func TestIsNamespace(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"json", true},
		{"owner/repo", false},
		{"", false},
		{"jsonx", false},
		{"JSON", false},
	}
	for _, c := range cases {
		if got := IsNamespace(c.path); got != c.want {
			t.Errorf("IsNamespace(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
