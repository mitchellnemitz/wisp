fn main() -> int {
  try {
    throw error("first")
  } catch (e) {
    throw error("escaped")
  } finally {
    print("cleanup")
  }
  return 0
}
