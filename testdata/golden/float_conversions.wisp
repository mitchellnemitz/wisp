fn main() -> int {
  let n: int = 2
  let f: float = to_float(n)
  print(to_string(f + 0.5))
  let g: float = to_float("-2")
  print(to_string(g * 3.0))
  print("${to_int(3.9)}")
  print("${to_int(-3.9)}")
  print(to_string(3.14))
  print(to_string(2.0))
  print("${to_bool(0.0)}")
  print("${to_bool(-0.0)}")
  print("${to_bool(0.000)}")
  print("${to_bool(1.5)}")
  return 0
}
