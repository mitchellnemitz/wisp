import "dict"
fn main() -> int {
  let m: {string: int} = { "a": 1, "b": 2 }
  m["c"] = 3
  m["a"] = 10
  print("a=${m["a"]}")
  print("hasb=${to_string(dict.has(m, "b"))}")
  print("hasz=${to_string(dict.has(m, "z"))}")
  print("nkeys=${length(dict.keys(m))}")
  return 0
}
