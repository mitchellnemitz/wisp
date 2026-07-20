import "string"
fn main() -> int {
    let xs: string[] = ["apple", "banana", "cherry"]
    let found: Optional[int] = string.index_of(xs, "banana")
    let absent: Optional[int] = string.index_of(xs, "mango")
    print(to_string(unwrap(found)))
    print(to_string(is_none(absent)))
    return 0
}
