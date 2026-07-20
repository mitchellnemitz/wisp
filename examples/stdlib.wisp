// A tour of the string standard library.
import "string"

fn main() -> int {
    let raw: string = "  Hello, World  "
    let s: string = string.trim(raw)
    print("trimmed: ${s}")
    print("lower: ${string.lower(s)}")
    print("upper: ${string.upper(s)}")
    print("replaced: ${string.replace(s, "World", "wisp")}")
    let parts: string[] = string.split("a,b,c", ",")
    print("parts: ${length(parts)}")
    print("joined: ${string.join(parts, " | ")}")
    print("starts with Hello: ${to_string(string.starts_with(s, "Hello"))}")
    print("index of comma: ${unwrap_or(string.index_of(s, ","), -1)}")
    print("divider: ${string.repeat("=", 10)}")
    return 0
}
