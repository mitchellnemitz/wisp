import "process"
fn main() -> int {
  try {
    print(process.run(["cat", "no_such_file_xyz"]))
  } catch (e) {
    print("caught run")
  }
  return 0
}
