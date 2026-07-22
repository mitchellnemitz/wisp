enum Answer: bool { No = false, Yes = true }
fn main() -> int {
  let a: Answer = Answer.Yes
  switch (a) {
    case Answer.No { print("no") }
    case Answer.Yes { print("yes") }
  }
  return 0
}
