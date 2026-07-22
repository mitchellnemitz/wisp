enum Expr { IntLit(int), Ident(string), Unit }
fn describe(e: Expr) -> string {
  match (e) {
    case IntLit(n) { return to_string(n) }
    case _ { return "other" }
  }
  return ""
}
fn main() -> int {
  print(describe(Expr.Ident("x")))
  print(describe(Expr.Unit))
  print(describe(Expr.IntLit(7)))
  return 0
}
