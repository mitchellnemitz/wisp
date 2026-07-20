fn add_context(e: error) -> error { return error("context: " + e.message) }
fn main() -> int {
    let ok: Result[int] = Ok(99)
    let r1: Result[int] = map_err(ok, add_context)
    print(to_string(unwrap(r1)))
    let e: error = error("orig")
    let err: Result[int] = Err(e)
    let r2: Result[int] = map_err(err, add_context)
    let transformed: error = unwrap_err(r2)
    print(transformed.message)
    return 0
}
