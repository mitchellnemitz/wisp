fn name(fd: int) -> string {
  switch (fd) {
    case stdout {
      return "out"
    }
    case stderr {
      return "err"
    }
    default {
      return "other"
    }
  }
}

fn main() -> int {
  print(name(1))
  print(name(2))
  print(name(3))
  return 0
}
