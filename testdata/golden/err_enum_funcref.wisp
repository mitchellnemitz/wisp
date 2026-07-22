enum Expr { IntLit(int), Unit }
fn apply(f: fn(int) -> Expr) -> Expr { return f(1) }
fn main() -> int {
  let e: Expr = apply(Expr.IntLit)
  return 0
}
