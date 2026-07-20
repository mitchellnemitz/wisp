import "json"
fn main() -> int {
  let v: json.Value = json.decode("{\"a\": 1, \"b\": [10, 20, 30]}")
  print(json.encode(unwrap(json.get(v, "a"))))
  print(to_string(is_none(json.get(v, "zzz"))))
  let arr: json.Value = unwrap(json.get(v, "b"))
  print(json.encode(unwrap(json.at(arr, 2))))
  print(to_string(is_none(json.at(arr, 9))))
  return 0
}
