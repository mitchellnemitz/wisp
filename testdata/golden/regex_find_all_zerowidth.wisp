import "regex"
import "string"
fn main() -> int {
  let xs: string[] = regex.find_all("aXbXc", "X*")
  print(string.join(xs, ","))
  print(to_string(length(xs)))
  return 0
}
