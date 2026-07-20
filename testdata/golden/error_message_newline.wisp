fn main() -> int {
  try {
    throw error("line1\n")
  } catch (e) {
    print(e.message + "END")
  }
  return 0
}
