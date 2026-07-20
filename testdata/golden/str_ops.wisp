import "string"
fn main() -> int {
    let s: string = "hello, world"
    print(string.substring(s, 7, 12))
    print(string.char_at(s, 0))
    print(to_string(unwrap_or(string.last_index_of("abcabc", "bc"), -1)))
    print(to_string(string.count("a.b.c.d", ".")))
    print(string.replace_first("x-y-z", "-", "+"))
    print("[" + string.trim_start("  hi ") + "]" + "[" + string.trim_end(" hi  ") + "]")
    print(string.trim_prefix("unhappy", "un") + " " + string.trim_suffix("running", "ning"))
    print("[" + string.pad_start("42", 5, "0") + "][" + string.pad_end("hi", 5, ".") + "]")
    print(string.join(string.lines("a\nb\nc\n"), "|") + " " + to_string(string.is_empty("")))
    return 0
}
