import "dict"
fn main() -> int {
  let mi: {int: string} = {1: "a", 2: "b"}
  let ms: {string: int} = {"x": 10, "y": 20}
  print("${unwrap_or(dict.get(mi, 1), "?")}")
  print("${unwrap_or(dict.get(ms, "y"), -1)}")
  return 0
}
