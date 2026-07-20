import "json"
fn main() -> int {
  print(json.encode(json.decode("9007199254740993")))
  print(json.encode(json.decode("0.1")))
  print(json.encode(json.decode("1e400")))
  print(json.encode(json.decode("-2.5E-10")))
  return 0
}
