struct Holder { s: string }
enum M { Wrap(Holder) }
fn main() -> int {
  print(debug(M.Wrap(Holder{s: "a;b\n"})))
  return 0
}
