import "json"
fn main() -> int {
  print(json.encode(json.decode("{ \"a\": [1, 2, {\"b\": true}], \"c\": null }")))
  return 0
}
