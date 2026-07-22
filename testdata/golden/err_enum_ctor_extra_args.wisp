enum Expr { IntLit(int), Unit }

fn main() -> int {
  let e: Expr = Expr.IntLit(1, 2)
  return 0
}
