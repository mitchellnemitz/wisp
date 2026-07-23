fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
  for (x in xs) {
    if (x == target) { return true }
  }
  return false
}

fn main() -> int {
  let hostile: string = "-a\\b'c\"d$e`f g\nh*i?j[k;l"
  let xs: string[] = [hostile, "plain"]
  print("${contains_eq(xs, hostile)}")
  return 0
}
