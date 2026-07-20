package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFSMetaEmittedShellByteIdentical is the byte-identity GATE for the
// fs-metadata work, reconstructed for the modules-only surface. It compiles a
// fixed program that uses existing fs builtins (file_exists, is_dir, cwd,
// make_dir, write_file), now namespaced (fs.*), but NONE of the nine new
// builtins (is_file, is_symlink, file_size, read_link, chmod, symlink, glob,
// temp_file, temp_dir). The namespaced delegate lowering is byte-identical to the
// pre-removal flat call, so the pre-removal snapshot still matches; a green run
// proves the new catalog entries and helpers are tree-shaken out of programs that
// do not call them (AC7 / N2).
//
// Regenerate the snapshot intentionally with:
// UPDATE_FSMETA_SNAPSHOT=1 go test ./internal/codegen -run TestFSMetaEmittedShellByteIdentical
func TestFSMetaEmittedShellByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  fs.make_dir("d")
  fs.write_file("d/f", "hello")
  let exists: bool = fs.file_exists("d/f")
  let is_d: bool = fs.is_dir("d")
  let is_d2: bool = fs.is_dir("d/f")
  let w: string = fs.cwd()
  print(to_string(exists))
  print(to_string(is_d))
  print(to_string(is_d2))
  print(w)
  return 0
}`
	got := compileNS(t, src, "fs")

	snap := filepath.Join("testdata", "fs_meta_byteidentity.sh")
	if os.Getenv("UPDATE_FSMETA_SNAPSHOT") == "1" {
		if err := os.MkdirAll(filepath.Dir(snap), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(snap, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
		return
	}

	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v (run with UPDATE_FSMETA_SNAPSHOT=1 to create)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("fs-meta emitted .sh changed (byte-identity gate failed).\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}
