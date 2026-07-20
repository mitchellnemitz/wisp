import "array"
fn double(x: int) -> int { return x * 2 }
fn main() -> int {
    let ok: Result[int] = Ok(5)
    let r1: Result[int] = array.map(ok, double)
    print(to_string(unwrap(r1)))
    let e: error = error("boom")
    let err: Result[int] = Err(e)
    let r2: Result[int] = array.map(err, double)
    let recovered: error = unwrap_err(r2)
    print(recovered.message)
    return 0
}
