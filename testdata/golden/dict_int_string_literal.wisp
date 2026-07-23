import "dict"
fn main() -> int {
  let mi: {int: string} = {1: "a", 2: "b"}
  let ms: {string: int} = {"x": 10, "y": 20}
  print("${dict.get_or(mi, 1, "?")}")
  print("${dict.get_or(ms, "y", -1)}")
  return 0
}
