import "string"
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  print("${string.contains(xs, 2)}")
  print("${string.contains(xs, 9)}")
  let ss: string[] = ["a", "b"]
  print("${string.contains(ss, "b")}")
  let bs: bool[] = [true]
  print("${string.contains(bs, false)}")
  let empty: int[] = []
  print("${string.contains(empty, 1)}")
  return 0
}
