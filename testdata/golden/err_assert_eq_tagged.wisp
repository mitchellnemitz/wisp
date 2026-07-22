enum Expr { IntLit(int), Unit }
fn main() -> int {
  assert_eq(Expr.Unit, Expr.Unit)
  return 0
}
