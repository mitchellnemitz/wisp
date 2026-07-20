fn main() -> int {
    let o: Optional[int] = Some(5)
    match (o) { case Some(_) { print("yes") } case None { print("no") } }
    return 0
}
