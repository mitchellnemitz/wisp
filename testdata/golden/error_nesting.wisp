fn main() -> int {
  try {
    try {
      throw error("inner")
    } catch (e) {
      throw error("from-inner")
    } finally {
      print("inner-finally")
    }
  } catch (e) {
    print("outer-catch:" + e.message)
  } finally {
    print("outer-finally")
  }
  return 0
}
