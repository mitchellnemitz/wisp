enum Answer: bool { No = false, Yes = true }

fn main() -> int {
  print(to_string(to_bool(Answer.Yes)))
  print(to_string(Answer.No == Answer.Yes))
  return 0
}
