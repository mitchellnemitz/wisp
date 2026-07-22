enum Expr { IntLit(int), Unit }
fn main() -> int {
  let xs: Expr[] = [Expr.Unit]
  assert_contains(xs, Expr.Unit)
  return 0
}
