enum Expr { IntLit(int), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  let i: int = to_int(e)
  return 0
}
