// Filesystem and process tour. Strictly read-only: it inspects the working
// directory and environment but never creates, moves, or removes anything, so
// it is safe to run from anywhere (the doctest runs it from the repo root).
import "env"
import "fs"
import "string"

fn main() -> int {
    print("cwd absolute: ${to_string(string.starts_with(fs.cwd(), "/"))}")
    print("cwd exists: ${to_string(fs.file_exists(fs.cwd()))}")
    print("sh resolvable: ${to_string(is_some(fs.which("sh")))}")
    print("PATH set: ${to_string(length(unwrap_or(env.get("PATH"), "")) > 0)}")
    print("cwd not empty: ${to_string(length(fs.list_dir(fs.cwd())) > 0)}")
    return 0
}
