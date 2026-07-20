import "string"
fn main() -> int {
    let xs: int[] = [10, 20, 30]
    let found: Optional[int] = string.index_of(xs, 20)
    let absent: Optional[int] = string.index_of(xs, 99)
    print(to_string(is_some(found)))
    print(to_string(unwrap(found)))
    print(to_string(is_none(absent)))
    return 0
}
