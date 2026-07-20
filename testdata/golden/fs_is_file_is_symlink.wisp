import "fs"
import "process"
fn main() -> int {
  fs.make_dir("d")
  fs.write_file("f", "x")
  let _lnout: string = process.run(["ln", "-s", "f", "s"])
  print(to_string(fs.is_file("f")))
  print(to_string(fs.is_file("d")))
  print(to_string(fs.is_file("s")))
  print(to_string(fs.is_file("m")))
  print(to_string(fs.is_symlink("s")))
  print(to_string(fs.is_symlink("f")))
  print(to_string(fs.is_symlink("d")))
  print(to_string(fs.is_symlink("m")))
  return 0
}
