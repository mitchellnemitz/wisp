import "dict"
import "string"
fn main() -> int {
    let novf: string[] = []

    // AC1: basic values/switches/positionals; the `=` form yields the same value.
    let (v1: {string: string}, s1: string[], p1: string[]) = parse_args(["--name", "ada", "--verbose", "f1", "f2"], ["--name"])
    print(unwrap_or(dict.get(v1, "--name"), "?"))
    print(string.join(s1, ","))
    print(string.join(p1, ","))
    let (v1b: {string: string}, _, _) = parse_args(["--name=ada"], ["--name"])
    print(unwrap_or(dict.get(v1b, "--name"), "?"))

    // AC2: `--` terminator drops itself; the rest are positionals.
    let (v2: {string: string}, s2: string[], p2: string[]) = parse_args(["--name", "ada", "--", "--not-a-flag", "x"], ["--name"])
    print(unwrap_or(dict.get(v2, "--name"), "?") + "|" + to_string(length(s2)) + "|" + string.join(p2, ","))

    // AC3: space-form consumes the flag-shaped next token; end-of-args omits.
    let (v3: {string: string}, _, _) = parse_args(["-o", "--weird"], ["-o"])
    print(unwrap_or(dict.get(v3, "-o"), "?"))
    let (v3b: {string: string}, s3b: string[], p3b: string[]) = parse_args(["-o"], ["-o"])
    print(to_string(dict.has(v3b, "-o")) + "|" + to_string(length(s3b)) + "|" + to_string(length(p3b)))

    // AC3a: `-o --` precedence, empty `--name=`, distinct `=` switches.
    let (v3c: {string: string}, _, p3c: string[]) = parse_args(["-o", "--", "x"], ["-o"])
    print(unwrap_or(dict.get(v3c, "-o"), "?") + "|" + string.join(p3c, ","))
    let (v3d: {string: string}, _, _) = parse_args(["--name="], ["--name"])
    print(to_string(dict.has(v3d, "--name")) + "|[" + unwrap_or(dict.get(v3d, "--name"), "?") + "]")
    let (_, s3e: string[], _) = parse_args(["--verbose", "--verbose=1"], novf)
    print(string.join(s3e, ","))

    // AC4: last occurrence wins; a lone `-` is a positional.
    let (v4: {string: string}, _, _) = parse_args(["--n", "a", "--n", "b"], ["--n"])
    print(unwrap_or(dict.get(v4, "--n"), "?"))
    let (_, _, p4: string[]) = parse_args(["-"], novf)
    print(string.join(p4, ",") + "|" + to_string(length(p4)))

    // Exact-string switch dedup.
    let (_, s5: string[], _) = parse_args(["--v", "--v"], novf)
    print(string.join(s5, ",") + "|" + to_string(length(s5)))
    return 0
}
