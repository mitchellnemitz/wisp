fn main() -> int {
  match (read_secret("pw: ")) {
    case Some(v) { print(v) }
    case None { print("none") }
  }
  return 0
}
