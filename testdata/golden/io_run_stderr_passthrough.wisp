import "process"
fn main() -> int {
  try {
    let out: string = process.run(["cat", "missing_for_stderr"])
    print("[${out}]")
  } catch (e) {
    print("done")
  }
  return 0
}
