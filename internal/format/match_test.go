package format

import (
	"strings"
	"testing"
)

func TestMatchCanonical(t *testing.T) {
	src := "fn main()->int{\n" +
		"let o:Optional[int]=Some(1)\n" +
		"match(o){case Some(x) {print(to_string(x))}case None {print(\"none\")}}\n" +
		"let r:Result[int]=Ok(7)\n" +
		"match(r){case Ok(v) {print(to_string(v))}case Err(e) {print(e.message)}}\n" +
		"return 0\n" +
		"}\n"
	want := "fn main() -> int {\n" +
		"    let o: Optional[int] = Some(1)\n" +
		"    match (o) {\n" +
		"        case Some(x) {\n" +
		"            print(to_string(x))\n" +
		"        }\n" +
		"        case None {\n" +
		"            print(\"none\")\n" +
		"        }\n" +
		"    }\n" +
		"    let r: Result[int] = Ok(7)\n" +
		"    match (r) {\n" +
		"        case Ok(v) {\n" +
		"            print(to_string(v))\n" +
		"        }\n" +
		"        case Err(e) {\n" +
		"            print(e.message)\n" +
		"        }\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Errorf("format mismatch:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

func TestMatchWildcardCanonical(t *testing.T) {
	src := "fn main()->int{\n" +
		"let o:Optional[int]=Some(1)\n" +
		"match(o){case Some(x) {print(to_string(x))}case _ {print(\"other\")}}\n" +
		"return 0\n}\n"
	want := "fn main() -> int {\n" +
		"    let o: Optional[int] = Some(1)\n" +
		"    match (o) {\n" +
		"        case Some(x) {\n" +
		"            print(to_string(x))\n" +
		"        }\n" +
		"        case _ {\n" +
		"            print(\"other\")\n" +
		"        }\n" +
		"    }\n" +
		"    return 0\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Errorf("format mismatch:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

func TestMatchIdempotent(t *testing.T) {
	srcs := []string{
		"fn main() -> int {\n    let o: Optional[int] = Some(1)\n    match (o) {\n        case Some(x) {\n            print(to_string(x))\n        }\n        case None {\n        }\n    }\n    return 0\n}\n",
		"fn main() -> int {\n    let r: Result[int] = Ok(1)\n    match (r) {\n        case Ok(v) {\n            print(to_string(v))\n        }\n        case Err(e) {\n            print(e.message)\n        }\n    }\n    return 0\n}\n",
		"fn main() -> int {\n    let o: Optional[int] = Some(1)\n    match (o) {\n        case Some(_) {\n            print(\"yes\")\n        }\n        case _ {\n        }\n    }\n    return 0\n}\n",
	}
	for _, src := range srcs {
		once := mustFormat(t, src)
		twice := mustFormat(t, once)
		if once != twice {
			t.Errorf("not idempotent:\n--once--\n%s\n--twice--\n%s", once, twice)
		}
		if !strings.Contains(once, "match (") {
			t.Errorf("formatter dropped the match statement:\n%s", once)
		}
	}
}
