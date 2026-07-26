// internal/fuzz/manifest.go
package fuzz

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed corpus/manifest.json
var canonicalManifestBytes []byte

// CanonicalManifestPath is the on-disk source of the embedded manifest (relative
// to the package dir), used only by the SC-016 negative test.
const CanonicalManifestPath = "corpus/manifest.json"

type BoundKind string

const (
	BoundProgramCount     BoundKind = "program_count"
	BoundGenerationBudget BoundKind = "generation_budget"
)

// ParseBoundKind validates a bound-kind string (from a CLI flag or any non-manifest
// path) and returns the typed value or an error. The manifest loader validates inline;
// the direct -seed / regen CLI paths, which construct a Provenance/ManifestEntry
// without going through LoadManifest, MUST route through this so a typo
// ("progam_count") fails loudly instead of producing a bogus BoundKind.
func ParseBoundKind(s string) (BoundKind, error) {
	switch BoundKind(s) {
	case BoundProgramCount, BoundGenerationBudget:
		return BoundKind(s), nil
	default:
		return "", fmt.Errorf("unknown bound_kind %q (want %q or %q)", s, BoundProgramCount, BoundGenerationBudget)
	}
}

type ManifestEntry struct {
	Seed       int64     `json:"seed"`
	BoundKind  BoundKind `json:"bound_kind"`
	BoundValue int       `json:"bound_value"`
}

// LoadManifest reads and validates an on-disk manifest. Read-only (FR-021).
func LoadManifest(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return parseManifest(path, data)
}

func parseManifest(src string, data []byte) ([]ManifestEntry, error) {
	var entries []ManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", src, err)
	}
	for i, e := range entries {
		if e.BoundKind != BoundProgramCount && e.BoundKind != BoundGenerationBudget {
			return nil, fmt.Errorf("manifest entry %d: unknown bound_kind %q", i, e.BoundKind)
		}
		if e.BoundValue <= 0 {
			return nil, fmt.Errorf("manifest entry %d: bound_value must be > 0, got %d", i, e.BoundValue)
		}
	}
	return entries, nil
}

// CanonicalManifestDigest is the SHA-256 of the embedded canonical bytes.
func CanonicalManifestDigest() string {
	sum := sha256.Sum256(canonicalManifestBytes)
	return hex.EncodeToString(sum[:])
}

// LoadCanonicalManifest parses the EMBEDDED canonical manifest -- location-
// independent and inherently read-only (FR-021). This is what baseline/regression
// checks (SC-001, SC-005, SC-009) run against.
func LoadCanonicalManifest() ([]ManifestEntry, error) {
	return parseManifest("embedded:corpus/manifest.json", canonicalManifestBytes)
}

// loadManifestWithDigest reads an on-disk manifest and refuses it unless its bytes
// hash to wantDigest -- the SC-016 guard that a mutated/regenerated manifest is
// refused, not silently used.
func loadManifestWithDigest(path, wantDigest string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != wantDigest {
		return nil, fmt.Errorf("manifest %s digest %s != canonical %s: refusing a mutated/regenerated manifest", path, got, wantDigest)
	}
	return parseManifest(path, data)
}
