fn safe_half(x: int) -> Optional[int] {
    if (x == 0) { return None }
    return Some(x / 2)
}
fn double_safe(x: int) -> Result[int] { return Ok(x * 2) }
fn main() -> int {
    let s: Optional[int] = Some(10)
    let r1: Optional[int] = and_then(s, safe_half)
    print(to_string(unwrap(r1)))
    let n: Optional[int] = None
    let r2: Optional[int] = and_then(n, safe_half)
    print(to_string(is_none(r2)))
    let z: Optional[int] = Some(0)
    let r3: Optional[int] = and_then(z, safe_half)
    print(to_string(is_none(r3)))
    let ok: Result[int] = Ok(3)
    let r4: Result[int] = and_then(ok, double_safe)
    print(to_string(unwrap(r4)))
    let e: error = error("fail")
    let err: Result[int] = Err(e)
    let r5: Result[int] = and_then(err, double_safe)
    print(to_string(is_err(r5)))
    return 0
}
