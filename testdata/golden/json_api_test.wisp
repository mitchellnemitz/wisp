import "json"

test ("encode round-trips decode") {
  assert_eq(json.encode(json.decode("{ \"a\": [1, 2] }")), "{\"a\":[1,2]}")
}

test ("decode[int] projects") {
  assert_eq(json.decode[int]("42"), 42)
}

test ("get returns Some for present key") {
  let v: json.Value = json.decode("{\"k\": 7}")
  assert_some(json.get(v, "k"))
}

test ("get returns None for absent key") {
  let v: json.Value = json.decode("{\"k\": 7}")
  assert_none(json.get(v, "missing"))
}

test ("as_string decodes unicode") {
  assert_eq(json.decode[string]("\"caf\\u00e9\""), "café")
}

test ("type_of classifies an array") {
  assert_eq(json.type_of(json.decode("[1]")), "array")
}
