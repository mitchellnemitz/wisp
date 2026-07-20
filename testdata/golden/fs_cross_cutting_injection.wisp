import "fs"
fn main() -> int {
  // === Leading-dash operands ===
  // is_file with a nonexistent leading-dash path: no "unknown option" error,
  // just false (the -- guard on test -f is implicit via double-quoting "$1").
  print(to_string(fs.is_file("-x")))

  // Create a real file whose name begins with '-' to exercise file_size,
  // read_link, and chmod on a leading-dash path.
  fs.write_file("-dash-file", "hi")
  let sz: int = fs.file_size("-dash-file")
  print(to_string(sz))
  print(to_string(is_none(fs.read_link("-dash-file"))))
  fs.chmod("-dash-file", "644")
  print(to_string(fs.is_file("-dash-file")))

  // symlink with a leading-dash TARGET (dangling link is fine for ln -s).
  fs.symlink("-target", "linkA")
  print(to_string(fs.is_symlink("linkA")))

  // symlink with a leading-dash LINK_PATH.
  fs.write_file("source_file", "x")
  fs.symlink("source_file", "-link")
  print(to_string(fs.is_symlink("-link")))

  // === Inert metacharacters ===
  // A path containing the full hostile set: $(...), backticks, ;, *, space,
  // and a NON-trailing newline. is_file returns false (path does not exist)
  // and nothing is executed (no pwned* file, empty stderr).
  let hostile_path: string = "$(touch pwned) `touch pwned2` ; * x\ny.txt"
  print(to_string(fs.is_file(hostile_path)))

  // === Hostile-target round-trip ===
  // symlink stores the target verbatim; read_link returns it byte-for-byte.
  // $() strips only the trailing newline; the embedded \n (non-trailing) is
  // preserved, so Some(v) == Some(hostileTarget) must hold.
  let hostile_target: string = "$(touch pwned3) `touch pwned4` ; * x\ny.txt"
  fs.symlink(hostile_target, "rt_link")
  let got: Optional[string] = fs.read_link("rt_link")
  match (got) {
    case Some(v) { print(to_string(v == hostile_target)) }
    case None { print("bad_none") }
  }

  // No side-effect file was created by any of the above.
  print(to_string(fs.file_exists("pwned")))
  print(to_string(fs.file_exists("pwned2")))
  print(to_string(fs.file_exists("pwned3")))
  print(to_string(fs.file_exists("pwned4")))
  return 0
}
