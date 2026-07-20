import "json"
fn main() -> int {
  print(to_string(json.decode[int]("-7")))
  print(to_string(json.decode[float]("2.5")))
  print(to_string(json.decode[bool]("false")))
  print(json.decode[string]("\"hi\\nthere\""))
  return 0
}
