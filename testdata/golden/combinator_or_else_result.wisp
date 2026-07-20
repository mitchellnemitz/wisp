fn recover(e: error) -> Result[int] { print(e.message)
    return Ok(0) }
fn main() -> int {
    let ok: Result[int] = Ok(7)
    let r1: Result[int] = or_else(ok, recover)
    print(to_string(unwrap(r1)))
    let e: error = error("bad input")
    let err: Result[int] = Err(e)
    let r2: Result[int] = or_else(err, recover)
    print(to_string(unwrap(r2)))
    return 0
}
