import "json"
fn main() -> int {
  print(json.encode(json.decode("{not: valid}")))
  return 0
}
