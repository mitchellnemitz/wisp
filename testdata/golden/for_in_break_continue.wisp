fn main() -> int {
  let xs: int[] = [1, 2, 3, 4, 5]
  for (x in xs) {
    if (x == 2) { continue }
    if (x == 4) { break }
    print(to_string(x))
  }
  return 0
}
