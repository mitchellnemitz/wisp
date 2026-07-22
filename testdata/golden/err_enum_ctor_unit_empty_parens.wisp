enum Expr { IntLit(int), Unit }

fn main() -> int {
  let e: Expr = Expr.Unit()
  return 0
}
