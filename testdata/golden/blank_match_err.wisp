fn main() -> int {
    let r: Result[int] = Err(error("boom"))
    match (r) { case Err(_) { print("caught") } case Ok(_) { print("ok") } }
    return 0
}
