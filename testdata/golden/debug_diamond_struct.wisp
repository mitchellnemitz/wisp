struct Point { x: int, y: int }
struct Pair { a: Point, b: Point }
fn main() -> int {
  let pr: Pair = Pair { a: Point { x: 1, y: 2 }, b: Point { x: 3, y: 4 } }
  print(debug(pr))
  return 0
}
