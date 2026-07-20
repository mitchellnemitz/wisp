package codegen

// TestIntMin_ArithRuntime proves that INT_MIN (-9223372036854775808) used as an
// arithmetic operand (both directly and via a const) evaluates correctly on
// every non-zsh shell returned by execShells(t).
//
// Coverage truth:
//   - dash and bash are always gated (dash is required; bash when on PATH).
//   - busybox-sh is gated too when the busybox binary is on PATH (execShells
//     adds it as "busybox-sh"; its arith for the spilled INT_MIN value is
//     correct -- probe-verified on docker busybox:latest v1.38).
//   - On a machine without busybox on PATH (e.g. this dev environment, where
//     busybox is docker-only) execShells omits it and this test does not
//     exercise it there.
//   - zsh is explicitly skipped: zsh $(( )) cannot represent 2^63 even from a
//     variable -- documented residual, not a regression.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIntMin_ArithRuntime(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "non_const",
			src: "fn main() -> int {\n" +
				"  let z: int = -9223372036854775808 + 1\n" +
				"  print(to_string(z))\n" +
				"  return 0\n" +
				"}",
			want: "-9223372036854775807\n",
		},
		{
			name: "const",
			src: "fn main() -> int {\n" +
				"  const M: int = -9223372036854775808\n" +
				"  let z: int = M + 1\n" +
				"  print(to_string(z))\n" +
				"  return 0\n" +
				"}",
			want: "-9223372036854775807\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := filepath.Join(dir, tc.name+".sh")
			if err := os.WriteFile(script, compile(t, tc.src), 0o755); err != nil {
				t.Fatal(err)
			}
			for _, sh := range execShells(t) {
				sh := sh
				// zsh $(( )) cannot represent 2^63 even from a variable -- documented residual.
				if sh.label == "zsh" {
					continue
				}
				t.Run(sh.label, func(t *testing.T) {
					args := append(append([]string{}, sh.args...), script)
					cmd := exec.Command(sh.bin, args...)
					out, err := cmd.Output()
					if err != nil {
						t.Fatalf("run: %v", err)
					}
					if string(out) != tc.want {
						t.Errorf("%s: stdout = %q, want %q", sh.label, out, tc.want)
					}
				})
			}
		})
	}
}
