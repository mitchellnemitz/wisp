enum T: string { Semi = ";", Star = "a*b", NL = "x\n", Quote = "a'b" }
fn classify(t: T) -> string {
  switch (t) {
    case T.Semi { return "semi" }
    case T.Star { return "star" }
    case T.NL { return "nl" }
    case T.Quote { return "quote" }
  }
  return ""
}
fn main() -> int {
  print(classify(T.Semi))
  print(classify(T.Star))
  print(classify(T.NL))
  print(classify(T.Quote))
  print(to_string(T.Semi == T.Star))
  return 0
}
