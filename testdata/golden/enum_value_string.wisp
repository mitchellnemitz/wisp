enum Dir: string { North, South = "s" }

fn main() -> int {
  print(to_string(Dir.North))
  print(to_string(Dir.South))
  print(to_string(Dir.North == Dir.South))
  return 0
}
