enum Expr { IntLit(int), Unit }
fn main() -> int {
  print(debug(Expr.IntLit(3)))
  print(debug(Expr.Unit))
  return 0
}
