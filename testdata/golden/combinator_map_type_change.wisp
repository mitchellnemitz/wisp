import "array"
fn to_str(x: int) -> string { return to_string(x) }
fn main() -> int {
    let o: Optional[int] = Some(42)
    let r1: Optional[string] = array.map(o, to_str)
    print(unwrap(r1))
    let n: Optional[int] = None
    let r2: Optional[string] = array.map(n, to_str)
    print(to_string(is_none(r2)))
    let ok: Result[int] = Ok(7)
    let r3: Result[string] = array.map(ok, to_str)
    print(unwrap(r3))
    return 0
}
