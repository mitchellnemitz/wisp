import "regex"
import "string"
fn main() -> int {
  let xs: string[] = regex.find_all("a1b22c333", "[0-9]+")
  print(string.join(xs, ","))
  print(to_string(length(xs)))
  return 0
}
