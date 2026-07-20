import "json"
fn main() -> int {
  print(json.encode(json.from_int(42)))
  print(json.encode(json.from_float(1.5)))
  print(json.encode(json.from_bool(true)))
  print(json.encode(json.from_string("he\"llo\\world")))
  print(json.encode(json.null()))
  return 0
}
