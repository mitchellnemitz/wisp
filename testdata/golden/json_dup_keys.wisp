import "json"
fn main() -> int {
  let v: json.Value = json.decode("{\"k\": 1, \"k\": 2}")
  print(json.encode(v))
  print(to_string(json.as_int(unwrap(json.get(v, "k")))))
  return 0
}
