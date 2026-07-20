import "array"
fn double(x: int) -> int { return x * 2 }
fn main() -> int {
    let s: Optional[int] = Some(5)
    let r1: Optional[int] = array.map(s, double)
    print(to_string(unwrap(r1)))
    let n: Optional[int] = None
    let r2: Optional[int] = array.map(n, double)
    print(to_string(is_none(r2)))
    return 0
}
