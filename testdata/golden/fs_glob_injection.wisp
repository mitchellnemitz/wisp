import "fs"
fn main() -> int {
  fs.make_dir("gdir")
  // A filename carrying the full hostile set: command substitution, backticks,
  // a semicolon, a glob star, a space, and a NON-trailing newline. If any of it
  // were re-evaluated as shell, a pwned file would appear and/or stderr would be
  // written. The glob match must round-trip the name verbatim (inert).
  let hostile: string = "gdir/$(touch pwned) `touch pwned2` ; rm -rf * x\ny.txt"
  fs.write_file(hostile, "")
  let got: string[] = fs.glob("gdir/*")
  print(to_string(length(got)))
  // The single matched path equals the path the file was created at, byte-for-byte.
  print(to_string(got[0] == hostile))
  print(to_string(fs.file_exists("pwned")))
  print(to_string(fs.file_exists("pwned2")))
  return 0
}
