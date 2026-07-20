fn main() -> int {
  try {
    throw error("orig")
  } catch (e) {
    print("caught")
    throw error("rethrown")
  } finally {
    print("finally")
  }
  return 0
}
