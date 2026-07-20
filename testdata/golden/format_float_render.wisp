import "string"
fn main() -> int {
  print(string.format_float(3.14159, 2))
  print(string.format_float(3.14159, 4))
  print(string.format_float(2.71828, 0))
  print(string.format_float(1.5, 3))
  print(string.format_float(0.0, 2))
  print(string.format_float(-2.5, 1))
  print(string.format_float(1.5, 10))
  return 0
}
