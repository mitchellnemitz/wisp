import "json"
fn main() -> int {
  print(debug(json.decode("[1, 2, {\"k\": true}]")))
  return 0
}
