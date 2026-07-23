fn subj() -> float {
  print("eval")
  return 2.5
}

fn main() -> int {
  switch (subj()) {
    case 1.0 { print("a") }
    case 2.5 { print("b") }
    default { print("d") }
  }
  return 0
}
