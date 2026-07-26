// internal/fuzz/manifest_test.go
package fuzz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestValidates(t *testing.T) {
	if _, err := LoadManifest(filepath.Join("testdata", "bad_manifest.json")); err == nil {
		t.Fatal("expected error for unknown bound_kind")
	}
	if _, err := LoadManifest(filepath.Join("testdata", "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadCanonicalManifest(t *testing.T) {
	entries, err := LoadCanonicalManifest()
	if err != nil {
		t.Fatalf("canonical manifest: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("canonical manifest empty")
	}
	for _, e := range entries {
		if e.BoundKind != BoundProgramCount && e.BoundKind != BoundGenerationBudget {
			t.Errorf("bad bound_kind %q", e.BoundKind)
		}
		if e.BoundValue <= 0 {
			t.Errorf("non-positive bound_value %d", e.BoundValue)
		}
	}
}

func TestLoadCanonicalRefusesMutatedManifest(t *testing.T) {
	// SC-016: a file whose bytes differ from the canonical digest is refused.
	tmp := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(tmp, []byte(`[{"seed":424242,"bound_kind":"program_count","bound_value":1}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifestWithDigest(tmp, CanonicalManifestDigest()); err == nil {
		t.Fatal("expected digest mismatch to be refused")
	}
}
