enum Expr { IntLit(int), Unit }
fn main() -> int {
  let e: Expr = Expr.Unit
  switch (e) {
    case Expr.Unit { }
    default { }
  }
  return 0
}
