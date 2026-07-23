struct Point { x: int, y: int }

fn eq2[T: comparable](a: T, b: T) -> bool {
  return a == b
}

fn main() -> int {
  let p: Point = Point { x: 1, y: 2 }
  print("${eq2(p, p)}")
  return 0
}
