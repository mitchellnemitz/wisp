// Demonstrates dir_name, base_name, and program_path.
//
// dir_name and base_name are pure string functions (no external process).
// program_path() returns $0 as the script was invoked; dir_name of that
// value gives the directory containing the running script, which is useful
// for locating sibling files at runtime.
import "fs"

fn main() -> int {
    print(fs.dir_name("/usr/local/bin/script.sh"))
    print(fs.base_name("/usr/local/bin/script.sh"))
    print(fs.dir_name("script.sh"))
    print(fs.base_name("script.sh"))
    // program_path() at runtime; its exact value depends on how the program
    // was invoked. Here we only assert it is non-empty and has a directory.
    let self_path: string = fs.program_path()
    let self_dir: string = fs.dir_name(self_path)
    print("self dir is non-empty: " + to_string(length(self_dir) > 0))
    return 0
}
