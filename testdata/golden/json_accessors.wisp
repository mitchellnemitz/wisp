import "json"
fn main() -> int {
  let v: json.Value = json.decode("{\"s\":\"x\",\"i\":7,\"f\":2.5,\"b\":true,\"n\":null,\"a\":[1]}")
  print(json.type_of(v))
  print(json.as_string(unwrap(json.get(v, "s"))))
  print(to_string(json.as_int(unwrap(json.get(v, "i")))))
  print(to_string(json.as_float(unwrap(json.get(v, "f")))))
  print(to_string(json.as_bool(unwrap(json.get(v, "b")))))
  print(json.type_of(unwrap(json.get(v, "n"))))
  print(json.type_of(unwrap(json.get(v, "a"))))
  return 0
}
