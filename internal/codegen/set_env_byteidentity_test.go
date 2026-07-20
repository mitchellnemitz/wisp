package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetEnv_HelperShape pins the __wisp_envname / __wisp_set_env /
// __wisp_unset_env helper shapes. Reconstructed with namespaced env.set / env.unset.
func TestSetEnv_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  env.set("FOO", "bar")
  env.unset("FOO")
  return 0
}`, "env"))
	for _, want := range []string{
		"__wisp_envname() {",
		"__wisp_set_env() {",
		"__wisp_unset_env() {",
		"LC_ALL=C",                    // locale pinned for byte-exact validation (else untested with ASCII names)
		"local __wisp_se_pair",        // spec R2: scratch var scoped with local
		`__wisp_se_pair="$2=$3"`,      // the NAME=VALUE pair word
		"# shellcheck disable=SC2163", // the required per-line disable
		`export "$__wisp_se_pair"`,    // export via the quoted pair word
		`unset "$2"`,
		"invalid environment variable name",
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// The value must never be re-evaluated: assert the unsafe UNQUOTED assignment
	// form `export $2=$3` (which would word-split/glob the value) is NOT emitted.
	// (Tightly tied to the exact unsafe form, not a broad `export $` scan.)
	if strings.Contains(sh, "export $2=$3") {
		t.Errorf("set_env must export via the quoted pair word, not the unquoted `export $2=$3` form")
	}
}

// TestSetEnv_NoUse_ByteIdentical: a program using the run family (process.run) but
// NOT set_env/unset_env emits shell byte-identical to before this feature, proving
// the three helpers are tree-shaken (AC5 / N2). The namespaced delegate lowering
// is byte-identical to the pre-removal flat call, so the pre-removal snapshot still
// matches. Regenerate with:
//
//	UPDATE_SET_ENV_SNAPSHOT=1 go test ./internal/codegen -run TestSetEnv_NoUse_ByteIdentical
func TestSetEnv_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  print(out)
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "set_env_byteidentity.sh")
	if os.Getenv("UPDATE_SET_ENV_SNAPSHOT") == "1" {
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
	if string(got) != string(want) {
		t.Errorf("emitted shell drifted from snapshot; re-mint with UPDATE_SET_ENV_SNAPSHOT=1 if intentional")
	}
	for _, helper := range []string{"__wisp_set_env", "__wisp_unset_env", "__wisp_envname"} {
		if strings.Contains(string(got), helper) {
			t.Errorf("%s leaked into a program that does not call set_env/unset_env", helper)
		}
	}
}
