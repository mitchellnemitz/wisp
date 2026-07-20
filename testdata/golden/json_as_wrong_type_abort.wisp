import "json"
fn main() -> int {
  print(to_string(json.as_int(json.from_string("x"))))
  return 0
}
