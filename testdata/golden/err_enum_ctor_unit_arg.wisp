enum Expr { IntLit(int), Unit }

fn main() -> int {
  let e: Expr = Expr.Unit(1)
  return 0
}
