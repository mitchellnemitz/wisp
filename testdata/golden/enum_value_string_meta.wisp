enum T: string { Semi = ";", Star = "a*b", NL = "x\n", Quote = "a'b" }

fn main() -> int {
  print(to_string(T.Semi))
  print(to_string(T.Star))
  print(to_string(T.NL))
  print(to_string(T.Quote))
  return 0
}
