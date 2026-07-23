import "array"
struct Point { x: int, y: int }
fn main() -> int {
  let arr: Point[] = [Point { x: 1, y: 2 }]
  let p: Point = Point { x: 1, y: 2 }
  let i: Optional[int] = array.index_of(arr, p)
  return 0
}
