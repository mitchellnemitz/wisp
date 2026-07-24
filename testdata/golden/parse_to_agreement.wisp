fn main() -> int {
    // int accepts
    print(to_string(unwrap(parse_int("0")) == to_int("0")))
    print(to_string(unwrap(parse_int("-0")) == to_int("-0")))
    print(to_string(unwrap(parse_int("+7")) == to_int("+7")))
    print(to_string(unwrap(parse_int("007")) == to_int("007")))
    print(to_string(unwrap(parse_int("-9223372036854775808")) == to_int("-9223372036854775808")))
    print(to_string(unwrap(parse_int("9223372036854775807")) == to_int("9223372036854775807")))
    // int rejects -> None
    print(to_string(is_none(parse_int("9223372036854775808"))))
    print(to_string(is_none(parse_int(""))))
    print(to_string(is_none(parse_int("1.0"))))
    print(to_string(is_none(parse_int("abc"))))
    print(to_string(is_none(parse_int(" 1"))))
    // float accepts
    print(to_string(unwrap(parse_float("3.14")) == to_float("3.14")))
    print(to_string(unwrap(parse_float("-0.0")) == to_float("-0.0")))
    // float rejects -> None
    print(to_string(is_none(parse_float(".5"))))
    print(to_string(is_none(parse_float("5."))))
    print(to_string(is_none(parse_float("1e3"))))
    print(to_string(is_none(parse_float(""))))
    print(to_string(is_none(parse_float("1.2.3"))))
    print(to_string(is_none(parse_float("x"))))
    // bool accepts
    print(to_string(unwrap(parse_bool("true")) == to_bool("true")))
    print(to_string(unwrap(parse_bool("false")) == to_bool("false")))
    // bool rejects -> None
    print(to_string(is_none(parse_bool("True"))))
    print(to_string(is_none(parse_bool("1"))))
    print(to_string(is_none(parse_bool("yes"))))
    print(to_string(is_none(parse_bool(""))))
    return 0
}
