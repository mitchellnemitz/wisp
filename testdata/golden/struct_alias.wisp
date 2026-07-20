struct Counter { n: int }
fn bump(c: Counter) -> void {
  c.n = c.n + 1
}
fn main() -> int {
  let a: Counter = Counter { n: 10 }
  let b: Counter = a
  b.n = 99
  bump(a)
  print("a=${a.n}")
  print("b=${b.n}")
  return 0
}
