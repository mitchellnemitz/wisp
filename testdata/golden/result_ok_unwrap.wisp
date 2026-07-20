fn main() -> int {
    let r: Result[int] = Ok(7)
    print("unwrap: ${to_string(unwrap(r))}")
    print("or: ${to_string(unwrap_or(r, 9))}")
    let e: Result[int] = Err(error("nope"))
    print("err or: ${to_string(unwrap_or(e, 9))}")
    return 0
}
