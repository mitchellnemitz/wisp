include "./lib/util.wisp" as util
import "acme/cfg" as cfg

fn banner(prefix: string = util.GREETING) -> string {
    return prefix
}

fn main() -> int {
    print(to_string(util.MAX_RETRIES))
    print(util.GREETING)
    if (util.FLAG) {
        print("on")
    }
    print(to_string(util.RATIO))
    print(banner())
    print(cfg.NAME)
    print(util.UNSAFE)
    return 0
}
