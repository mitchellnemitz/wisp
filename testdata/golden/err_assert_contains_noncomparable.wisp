struct Point { x: int, y: int }
fn main() -> int {
  let arr: Point[] = [Point { x: 1, y: 2 }]
  assert_contains(arr, Point { x: 1, y: 2 })
  return 0
}
