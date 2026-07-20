import "process"
import "string"
fn main() -> int {
  // An env value that ends with a newline byte. Command substitution $(...) would
  // strip the trailing newline (yielding "abc", 3 bytes); plain concatenation
  // preserves it ("abc\n", 4 bytes). We measure the byte count of $V in the child
  // and assert it equals 4.
  let e: {string: string} = {"V": "abc\n"}
  let out: string = process.run_env(["sh", "-c", "printf %s \"$V\" | wc -c"], e)
  let n: int = to_int(string.trim(out))
  print(to_string(n == 4))
  return 0
}
