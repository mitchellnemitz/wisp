fn main() -> int {
  try {
    throw error_with(5, "oops")
  } catch (e) {
    print(e.message)
    print(to_string(e.code))
  }
  return 0
}
