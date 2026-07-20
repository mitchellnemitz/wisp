fn make_default() -> Optional[int] { return Some(42) }
fn main() -> int {
    let s: Optional[int] = Some(7)
    let r1: Optional[int] = or_else(s, make_default)
    print(to_string(unwrap(r1)))
    let n: Optional[int] = None
    let r2: Optional[int] = or_else(n, make_default)
    print(to_string(unwrap(r2)))
    return 0
}
