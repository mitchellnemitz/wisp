import "regex"
fn main() -> int {
  let xs: string[] = regex.find_all("a\nb", "a.b")
  print(to_string(length(xs)))
  print(to_string(length(xs[0])))
  print("[" + xs[0] + "]")
  return 0
}
