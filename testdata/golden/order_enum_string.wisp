enum Size: string { S = "a", M = "b", L = "c" }
fn main() -> int {
  print(to_string(Size.S < Size.M))
  print(to_string(Size.L < Size.S))
  print(to_string(Size.M <= Size.M))
  print(to_string(Size.L > Size.S))
  print(to_string(Size.S >= Size.L))
  return 0
}
