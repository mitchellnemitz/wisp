// String round-out standard library helpers.
import "string"

fn main() -> int {
    let s: string = "Hello, World"
    print(string.substring(s, 0, 5))
    print(string.char_at(s, 7))
    print("index: ${to_string(unwrap_or(string.last_index_of(s, "o"), -1))}")
    print("count l: ${to_string(string.count(s, "l"))}")
    print(string.replace_first(s, "o", "0"))
    print("[${string.trim_start("   spaced")}]")
    print("[${string.trim_end("spaced   ")}]")
    print(string.trim_prefix("unhappy", "un"))
    print(string.trim_suffix("running", "ning"))
    print("[${string.pad_start("42", 5, "0")}]")
    print("[${string.pad_end("hi", 5, ".")}]")
    print("empty: ${to_string(string.is_empty(""))}")
    let text: string = "alpha\nbeta\ngamma\n"
    let ls: string[] = string.lines(text)
    print("lines: ${to_string(length(ls))}")
    print(string.join(ls, " / "))
    return 0
}
