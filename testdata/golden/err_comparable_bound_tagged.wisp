enum Expr { IntLit(int), Unit }
fn eq2[T: comparable](a: T, b: T) -> bool {
  return a == b
}
fn main() -> int {
  print(to_string(eq2(Expr.Unit, Expr.Unit)))
  return 0
}
