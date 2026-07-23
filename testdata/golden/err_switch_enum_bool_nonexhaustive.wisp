enum Answer: bool { No = false, Yes = true }
fn main() -> int {
  let a: Answer = Answer.Yes
  switch (a) {
    case Answer.Yes { }
  }
  return 0
}
