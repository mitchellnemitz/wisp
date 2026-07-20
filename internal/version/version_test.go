package version

import "testing"

func TestNumberNonEmpty(t *testing.T) {
	if Number == "" {
		t.Fatal("version.Number must not be empty")
	}
}
