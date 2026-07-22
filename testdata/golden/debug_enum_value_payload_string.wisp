enum Token: string { Semi = ";\n" }
enum Wrap { W(Token) }
fn main() -> int {
  print(debug(Wrap.W(Token.Semi)))
  return 0
}
