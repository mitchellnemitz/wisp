import "array"
fn add(acc: int, x: int) -> int { return acc + x }
fn sub(acc: int, x: int) -> int { return acc - x }
fn main() -> int {
  let xs: int[] = [1, 2, 3, 4]
  print("${array.reduce(xs, 0, add)}")
  let ys: int[] = [1, 2, 3]
  print("${array.reduce(ys, 100, sub)}")
  let e: int[] = []
  print("${array.reduce(e, 42, add)}")
  return 0
}
