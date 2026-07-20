import "fs"
fn main() -> int {
  try {
    print(fs.read_file("does_not_exist"))
  } catch (e) {
    print("caught")
  }
  return 0
}
