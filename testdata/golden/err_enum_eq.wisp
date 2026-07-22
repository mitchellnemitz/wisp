enum Expr { IntLit(int), Unit }
fn main() -> int {
  let a: Expr = Expr.Unit
  let b: Expr = Expr.Unit
  if (a == b) { print("eq") }
  return 0
}
