fn faulty(x: int) -> Optional[int] {
    let n: Optional[int] = None
    let bad: int = unwrap(n)
    return Some(bad)
}
fn main() -> int {
    try {
        let r: Optional[int] = and_then(Some(1), faulty)
        print("no fault")
    } catch (e) {
        print("caught")
    }
    return 0
}
