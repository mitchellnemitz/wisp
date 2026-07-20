fn main() -> int {
  let x: int = 5
  let y: int = -x
  print(to_string(y))
  print(to_string(-5))
  print(to_string(-9223372036854775807))
  print(to_string(-0))
  print(to_string(-007))
  return 0
}
