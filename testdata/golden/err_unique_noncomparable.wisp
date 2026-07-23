import "array"
struct Point { x: int, y: int }
fn main() -> int {
  let arr: Point[] = [Point { x: 1, y: 2 }]
  let u: Point[] = array.unique(arr)
  return 0
}
