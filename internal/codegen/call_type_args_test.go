package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestCallTypeArgs_ErasedIdenticalToInferred: an explicit-type-arg call to an
// erased (unbounded) generic lowers byte-identically to the inferred form. This is
// the spec's no-codegen-divergence invariant for the erased case.
func TestCallTypeArgs_ErasedIdenticalToInferred(t *testing.T) {
	const decl = "fn identity[T](x: T) -> T {\n  return x\n}\n"
	explicit := compile(t, decl+"fn main() -> int {\n  return identity[int](42)\n}\n")
	inferred := compile(t, decl+"fn main() -> int {\n  return identity(42)\n}\n")
	if !bytes.Equal(explicit, inferred) {
		t.Fatalf("erased explicit != inferred:\n--explicit--\n%s\n--inferred--\n%s", explicit, inferred)
	}
}

// TestCallTypeArgs_NumericIdenticalToInferred: an explicit-type-arg call to a
// numeric-bounded (monomorphized) generic lowers byte-identically to the inferred
// form -- same TypeSubst, same specialization suffix.
func TestCallTypeArgs_NumericIdenticalToInferred(t *testing.T) {
	const decl = "fn add[T: numeric](a: T, b: T) -> T {\n  return a + b\n}\n"
	explicit := compile(t, decl+"fn main() -> int {\n  print(to_string(add[float](1.5, 2.5)))\n  return 0\n}\n")
	inferred := compile(t, decl+"fn main() -> int {\n  print(to_string(add(1.5, 2.5)))\n  return 0\n}\n")
	if !bytes.Equal(explicit, inferred) {
		t.Fatalf("numeric explicit != inferred:\n--explicit--\n%s\n--inferred--\n%s", explicit, inferred)
	}
}

// TestCallTypeArgs_NestedTypeVar: an explicit type arg that is a caller type
// variable, `make[U]()` inside a concretely-instantiated numeric generic, lowers
// and runs correctly (transitive $U resolution via the monomorphizer BFS).
func TestCallTypeArgs_NestedTypeVar(t *testing.T) {
	src := "fn make[T: numeric](s: T) -> T {\n  return s + s\n}\n" +
		"fn outer[U: numeric](seed: U) -> U {\n  let z: U = make[U](seed)\n  return z\n}\n" +
		"fn main() -> int {\n  print(to_string(outer[int](7)))\n  return 0\n}\n"
	script := compile(t, src)
	stdout, _, code := run(t, script)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if stdout != "14\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "14\n")
	}
}

// TestCallTypeArgs_NoUse_ByteIdentical: a program that does NOT use explicit type
// args must emit byte-identical shell to the snapshot. Codegen is untouched by this
// feature (it reads CallInfo, never CallExpr.TypeArgs), so this guards against a
// future regression altering non-type-arg output.
// Regenerate: UPDATE_CALL_TYPE_ARGS_SNAPSHOT=1 go test ./internal/codegen -run TestCallTypeArgs_NoUse_ByteIdentical
func TestCallTypeArgs_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn identity[T](x: T) -> T {
	return x
}

fn add[T: numeric](a: T, b: T) -> T {
	return a + b
}

fn main() -> int {
	let a: int = identity(41) + 1
	let f: float = add(1.5, 2.5)
	print("${a}")
	print("${f}")
	return 0
}
`
	got := compile(t, src)
	snap := filepath.Join("testdata", "call_type_args_byteidentity.sh")
	if os.Getenv("UPDATE_CALL_TYPE_ARGS_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v", snap, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("no-type-args program .sh changed (byte-identity gate failed).\n--got--\n%s\n--want--\n%s", got, want)
	}
}
