enum Expr { IntLit(int), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  let b: bool = to_bool(e)
  return 0
}
