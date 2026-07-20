fn main() -> int {
  match (read_secret("pw: ")) {
    case Some(v) { print("some:" + v) }
    case None { print("none") }
  }
  return 0
}
