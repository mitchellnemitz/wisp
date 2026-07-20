import "array"
fn positive(x: int) -> bool { return x > 0 }
fn main() -> int {
    let s1: Optional[int] = Some(3)
    let r1: Optional[int] = array.filter(s1, positive)
    print(to_string(is_some(r1)))
    let s2: Optional[int] = Some(-1)
    let r2: Optional[int] = array.filter(s2, positive)
    print(to_string(is_none(r2)))
    let n: Optional[int] = None
    let r3: Optional[int] = array.filter(n, positive)
    print(to_string(is_none(r3)))
    return 0
}
