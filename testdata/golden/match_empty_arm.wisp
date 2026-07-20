fn main() -> int {
  let a: Optional[int] = Some(7)
  match (a) {
    case Some(v) { print(to_string(v)) }
    case None {}
  }
  let b: Optional[int] = None
  match (b) {
    case Some(_) { print("unexpected") }
    case None {}
  }
  let r: Result[int] = Ok(3)
  match (r) {
    case Ok(_) {}
    case Err(_) { print("err") }
  }
  match (a) {
    case Some(_) { print("wild-some") }
    case _ {}
  }
  print("done")
  return 0
}
