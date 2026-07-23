fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
  for (x in xs) {
    if (x == target) { return true }
  }
  return false
}

fn main() -> int {
  let fs: float[] = [1.0, 2.5]
  print("${contains_eq(fs, 1.00)}")
  let ss: string[] = ["a", "b"]
  print("${contains_eq(ss, "b")}")
  let bs: bool[] = [true]
  print("${contains_eq(bs, false)}")
  return 0
}
