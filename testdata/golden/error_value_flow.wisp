fn describe(e: error) -> string {
  return e.message
}
fn make(msg: string) -> error {
  return error(msg)
}
fn main() -> int {
  let e: error = make("hello")
  print(describe(e))
  return 0
}
