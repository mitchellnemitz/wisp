import "dict"
enum Code: int { A = 1, B = 2 }

fn main() -> int {
  let b: {bool: int} = {true: 1, false: 2}
  let f: {float: string} = {1.5: "a", 2.5: "b"}
  let e: {Code: int} = {Code.A: 10, Code.B: 20}
  print("${dict.get_or(b, true, -1)}")
  print("${dict.get_or(f, 2.5, "?")}")
  print("${dict.get_or(e, Code.B, -1)}")
  return 0
}
