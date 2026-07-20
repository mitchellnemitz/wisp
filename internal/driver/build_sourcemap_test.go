package driver

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildSourceMapWritesBothFiles: with sourceMap=true, Build writes out.sh
// and out.sh.map.
func TestBuildSourceMapWritesBothFiles(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hello.sh")
	var stderr bytes.Buffer
	code := Build("hello.wisp", helloSrc, out, true, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf(".sh not written: %v", err)
	}
	mapPath := out + ".map"
	mb, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatalf(".sh.map not written: %v", err)
	}
	var sm sourceMap
	if err := json.Unmarshal(mb, &sm); err != nil {
		t.Fatalf(".sh.map invalid JSON: %v", err)
	}
	if sm.Version != 1 {
		t.Errorf("map version = %d, want 1", sm.Version)
	}
}

// TestBuildNoSourceMapWritesOnlySh: without the flag only the .sh is written.
func TestBuildNoSourceMapWritesOnlySh(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hello.sh")
	var stderr bytes.Buffer
	if code := Build("hello.wisp", helloSrc, out, false, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf(".sh not written: %v", err)
	}
	if _, err := os.Stat(out + ".map"); !os.IsNotExist(err) {
		t.Fatalf(".sh.map should not exist without the flag: %v", err)
	}
}

// TestBuildShByteIdenticalWithAndWithoutMap: the .sh bytes are identical whether
// or not --source-map is set (the map is a sidecar; AC 1).
func TestBuildShByteIdenticalWithAndWithoutMap(t *testing.T) {
	dir := t.TempDir()
	outA := filepath.Join(dir, "a.sh")
	outB := filepath.Join(dir, "b.sh")
	var se bytes.Buffer
	if code := Build("p.wisp", helloSrc, outA, false, &se); code != 0 {
		t.Fatalf("build A exit %d: %q", code, se.String())
	}
	if code := Build("p.wisp", helloSrc, outB, true, &se); code != 0 {
		t.Fatalf("build B exit %d: %q", code, se.String())
	}
	a, _ := os.ReadFile(outA)
	b, _ := os.ReadFile(outB)
	if !bytes.Equal(a, b) {
		t.Fatalf(".sh bytes differ with vs without --source-map")
	}
}

// TestBuildSourceMapUnwritablePathStillWritesSh: when the .sh.map write fails,
// the .sh remains as the authoritative artifact and Build exits non-zero (spec
// section 6 partial-failure, AC 6).
func TestBuildSourceMapUnwritablePathStillWritesSh(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "hello.sh")
	// Pre-create out.sh.map as a directory so writing it as a file fails.
	if err := os.Mkdir(out+".map", 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := Build("hello.wisp", helloSrc, out, true, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit when the map write fails")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf(".sh must still be present after a failed map write: %v", err)
	}
	if stderr.Len() == 0 {
		t.Error("expected an error on stderr for the failed map write")
	}
}

// TestBuildSourceMapStoresPathsVerbatim: a relative input path and the -o path
// are stored unresolved (AC 5).
func TestBuildSourceMapStoresPathsVerbatim(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.sh")
	var stderr bytes.Buffer
	if code := Build("rel/in.wisp", helloSrc, out, true, &stderr); code != 0 {
		t.Fatalf("exit %d: %q", code, stderr.String())
	}
	mb, err := os.ReadFile(out + ".map")
	if err != nil {
		t.Fatal(err)
	}
	var sm sourceMap
	if err := json.Unmarshal(mb, &sm); err != nil {
		t.Fatal(err)
	}
	if sm.Source != "rel/in.wisp" {
		t.Errorf("source = %q, want rel/in.wisp (unresolved)", sm.Source)
	}
	if sm.Generated != out {
		t.Errorf("generated = %q, want %q (the -o value verbatim)", sm.Generated, out)
	}
}
