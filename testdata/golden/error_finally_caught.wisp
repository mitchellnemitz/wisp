fn main() -> int {
  try {
    throw error("boom")
  } catch (e) {
    print("caught:" + e.message)
  } finally {
    print("finally")
  }
  return 0
}
