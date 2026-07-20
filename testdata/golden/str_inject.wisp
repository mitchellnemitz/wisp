import "string"
fn main() -> int {
    print(string.replace_first("a*b", "*", "X"))
    print(to_string(string.count("a*a*", "*")))
    print(string.trim_prefix("$x", "$"))
    print("[" + string.pad_start("z", 3, "*?") + "]")
    print(to_string(unwrap_or(string.last_index_of("a[b]c", "["), -1)))
    return 0
}
