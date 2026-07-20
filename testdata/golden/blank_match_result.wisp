fn main() -> int {
    let r: Result[int] = Ok(7)
    match (r) { case Ok(_) { print("yes") } case Err(_) { print("no") } }
    return 0
}
