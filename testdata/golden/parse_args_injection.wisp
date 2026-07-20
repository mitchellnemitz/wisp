import "dict"
fn main() -> int {
    // A value-flag value and a positional carrying shell metacharacters:
    // command substitution, backticks, a semicolon, a glob star, spaces, and an
    // embedded newline. parse_args must preserve them all literally (N1).
    let danger: string = "$(touch pwned); `id`; a * b"
    let nl: string = "line1\nline2"
    let (vals: {string: string}, _, pos: string[]) = parse_args(["--cmd", danger, nl, "--also=$(rm -rf /)"], ["--cmd", "--also"])
    print(unwrap_or(dict.get(vals, "--cmd"), "?"))
    print(unwrap_or(dict.get(vals, "--also"), "?"))
    print(pos[0])
    print(to_string(length(pos)))
    return 0
}
