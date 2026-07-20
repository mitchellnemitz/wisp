fn main() -> int {
  try {
    throw error("plain")
  } catch (e) {
    print(e.message)
    print(to_string(e.code))
  }
  return 0
}
