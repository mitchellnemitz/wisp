fn label(b: bool) -> string {
  switch (b) {
    case true { return "yes" }
    case false { return "no" }
  }
  return ""
}
fn main() -> int {
  print(label(true))
  print(label(false))
  return 0
}
