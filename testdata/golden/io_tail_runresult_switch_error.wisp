import "process"
fn main() -> int {
  let r: RunResult = process.run_full(["true"])
  switch (r) {
    default {}
  }
  return 0
}
