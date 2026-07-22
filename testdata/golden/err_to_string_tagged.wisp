enum Expr { IntLit(int), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  let s: string = to_string(e)
  return 0
}
