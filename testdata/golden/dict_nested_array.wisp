import "array"
fn main() -> int {
  let m: {string: int[]} = { "a": [1, 2], "b": [3, 4, 5] }
  print("alen=${length(m["a"])}")
  print("b2=${m["b"][2]}")
  array.push(m["a"], 9)
  print("alen2=${length(m["a"])}")
  return 0
}
