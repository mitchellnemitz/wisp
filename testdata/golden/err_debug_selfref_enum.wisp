struct BinaryPayload { op: string, left: Expr, right: Expr }
enum Expr { IntLit(int), Binary(BinaryPayload) }
fn main() -> int {
  let e: Expr = Expr.IntLit(1)
  let _ : string = debug(e)
  return 0
}
