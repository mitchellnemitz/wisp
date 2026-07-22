enum Expr { IntLit(int), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  e[0] = Expr.Unit
  return 0
}
