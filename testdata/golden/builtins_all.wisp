import "string"
fn main() -> int {
  let seven: int = to_int("7")
  let n: int = length("hello")
  let bt: bool = to_bool("true")
  print("plain")
  print(to_string(42))
  print(to_string(true))
  print(to_string("kept"))
  print("${seven + 1}")
  print("${to_bool(0)}")
  print("${to_bool(1)}")
  print("${bt}")
  print("${n}")
  print(string.lower("MixedCase"))
  print(string.upper("MixedCase"))
  print(string.trim("  spaced  "))
  print(string.replace("a-b-c", "-", "_"))
  return 0
}
