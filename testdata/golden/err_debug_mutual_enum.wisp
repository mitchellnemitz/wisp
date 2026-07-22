enum A { Ga(B), Ba(int) }
enum B { Gb(A), Bb(int) }
fn main() -> int {
  let a: A = A.Ba(1)
  let _ : string = debug(a)
  return 0
}
