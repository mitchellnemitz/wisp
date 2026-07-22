enum Expr { IntLit(int), Unit }

fn main() -> int {
  let e: Expr = Expr.IntLit()
  return 0
}
