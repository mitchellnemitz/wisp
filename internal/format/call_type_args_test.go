package format

import (
	"strings"
	"testing"
)

// TestFormatCallTypeArgs renders explicit call-site type arguments as
// `name[T1, T2](args)` and round-trips idempotently, including composite args,
// an alias arg (printed as the alias name), and the qualified form (which the
// golden idempotency sweep skips for *.dir fixtures, so it is asserted here).
func TestFormatCallTypeArgs(t *testing.T) {
	cases := []struct {
		src  string
		want string // a substring the formatted output must contain
	}{
		{"f[int](x)", "f[int](x)"},
		{"f[int,string](x)", "f[int, string](x)"},
		{"f[int[]](x)", "f[int[]](x)"},
		{"f[{string:int}](x)", "f[{string: int}](x)"},
		{"f[fn(int)->bool](x)", "f[fn(int) -> bool](x)"},
		{"f[Box[int]](x)", "f[Box[int]](x)"},
		{"ns.decode[int](x)", "ns.decode[int](x)"},
	}
	for _, c := range cases {
		src := "fn main() -> int {\n" + c.src + "\nreturn 0\n}\n"
		got := mustFormat(t, src)
		if !strings.Contains(got, c.want) {
			t.Errorf("format %q:\n got:\n%s\n want substring:\n%s", c.src, got, c.want)
		}
		if mustFormat(t, got) != got {
			t.Errorf("not idempotent for %q:\n%s", c.src, got)
		}
	}
}

// TestFormatCallNoTypeArgsUnchanged: a plain call must not gain an empty `[]`.
func TestFormatCallNoTypeArgsUnchanged(t *testing.T) {
	src := "fn main() -> int {\n    f(x)\n    return 0\n}\n"
	if got := mustFormat(t, src); got != src {
		t.Errorf("plain call changed:\n got:\n%s\n want:\n%s", got, src)
	}
}

// TestFormatCallTypeArgsTrailingComment: a trailing comment stays attached after
// a type-arg call (rightmostLine unaffected; type args are never the rightmost).
func TestFormatCallTypeArgsTrailingComment(t *testing.T) {
	src := "fn main() -> int {\n    f[int](x) // c\n    return 0\n}\n"
	got := mustFormat(t, src)
	if !strings.Contains(got, "f[int](x) // c") {
		t.Errorf("trailing comment misplaced:\n%s", got)
	}
	if mustFormat(t, got) != got {
		t.Errorf("not idempotent:\n%s", got)
	}
}
