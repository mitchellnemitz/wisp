fn main() -> int {
    print(to_string(is_none(parse_int("$(touch /tmp/wisp_pwned_pi)"))))
    print(to_string(is_none(parse_float("`touch /tmp/wisp_pwned_pf`"))))
    print(to_string(is_none(parse_bool("x; rm -rf /tmp/nope"))))
    print(to_string(is_none(parse_int("*"))))
    print(to_string(is_none(parse_bool("true|false"))))
    return 0
}
