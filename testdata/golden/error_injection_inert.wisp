fn main() -> int {
  try {
    throw error("$(touch SENTINEL_INJECT) `echo no`")
  } catch (e) {
    print(e.message)
  }
  return 0
}
