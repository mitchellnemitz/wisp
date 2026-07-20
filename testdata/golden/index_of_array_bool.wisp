import "string"
fn main() -> int {
    let xs: bool[] = [false, true, false]
    let found: Optional[int] = string.index_of(xs, true)
    let absent: Optional[int] = string.index_of(xs, true)
    print(to_string(unwrap(found)))
    print(to_string(is_some(absent)))
    return 0
}
