include "./lib/e.wisp" as e

fn main() -> int {
  let x: e.Expr = e.Expr.IntLit(7)
  match (x) {
    case IntLit(n) { print(to_string(n)) }
    case Unit { print("u") }
  }
  let y: e.Expr = e.Expr.Unit
  match (y) {
    case IntLit(n) { print(to_string(n)) }
    case Unit { print("u") }
  }
  return 0
}
