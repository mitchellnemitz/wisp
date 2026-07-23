fn pair_eq[A: comparable, B: comparable](a1: A, a2: A, b1: B, b2: B) -> bool {
  return a1 == a2 && b1 == b2
}

fn main() -> int {
  print("${pair_eq(1.0, 1.00, "x", "x")}")
  print("${pair_eq("p", "p", 2.5, 2.50)}")
  print("${pair_eq("m", "m", "n", "n")}")
  return 0
}
