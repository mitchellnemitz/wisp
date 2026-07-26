// internal/fuzz/corpus_test.go
package fuzz

import (
	"os"
	"testing"
)

func TestPersistAndLoadCandidate(t *testing.T) {
	dir := t.TempDir()
	f := Finding{
		Provenance: Provenance{Seed: 7, BoundKind: BoundProgramCount, BoundValue: 100, ProgramIndex: 12},
		Source:     "fn main() -> int {\n  return 0\n}\n",
		Runs:       []ShellRun{{Label: "dash", Stdout: []byte("a\n")}, {Label: "bash", Stdout: []byte("b\n")}},
		Detail:     "dash vs bash",
	}
	path, err := PersistCandidate(dir, f)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("not written: %v", err)
	}
	got, err := LoadCorpus(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Provenance.Seed != 7 || got[0].Source != f.Source {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPersistOverwritesSameProvenance(t *testing.T) {
	dir := t.TempDir()
	f := Finding{Provenance: Provenance{Seed: 1, BoundKind: BoundProgramCount, BoundValue: 10, ProgramIndex: 3}, Source: "raw"}
	if _, err := PersistCandidate(dir, f); err != nil {
		t.Fatal(err)
	}
	f.Source, f.Shrunk = "shrunk", true
	if _, err := PersistCandidate(dir, f); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadCorpus(dir)
	if len(got) != 1 || got[0].Source != "shrunk" || !got[0].Shrunk {
		t.Fatalf("expected single overwritten shrunk record, got %+v", got)
	}
}
