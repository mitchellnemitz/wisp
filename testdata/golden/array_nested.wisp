struct Point { x: int, y: int }
fn main() -> int {
  let g: int[][] = [[1, 2], [3, 4, 5]]
  print("rowlen=${length(g[1])}")
  print("g12=${g[1][2]}")
  let ps: Point[] = [Point { x: 1, y: 2 }, Point { x: 3, y: 4 }]
  ps[0].x = 100
  print("p0x=${ps[0].x}")
  print("p1y=${ps[1].y}")
  return 0
}
