fn label(msg: string, prefix: string = "[info] ") -> string {
  return prefix + msg
}
fn main() -> int {
  print(label("hello"))
  print(label("hello", "[warn] "))
  return 0
}
