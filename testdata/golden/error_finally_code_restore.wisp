fn main() -> int {
  try {
    try {
      throw error_with(42, "orig")
    } catch (e) {
      throw error_with(42, "rethrown")
    } finally {
      try {
        throw error_with(7, "inner")
      } catch (e2) {
        print("inner-caught:" + to_string(e2.code))
      }
    }
  } catch (eo) {
    print("outer-msg:" + eo.message)
    print("outer-code:" + to_string(eo.code))
  }
  return 0
}
