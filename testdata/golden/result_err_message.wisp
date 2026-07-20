fn main() -> int {
    let e: Result[int] = Err(error("boom"))
    print("is_err: ${to_string(is_err(e))}")
    print("is_ok: ${to_string(is_ok(e))}")
    print("msg: ${unwrap_err(e).message}")
    return 0
}
