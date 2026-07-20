import "json"
fn main() -> int {
  print(json.decode[string]("\"caf\\u00e9\""))
  print(json.decode[string]("\"\\u20ac\""))
  print(json.decode[string]("\"\\ud83d\\ude00\""))
  return 0
}
