fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
	for (x in xs) {
		if (x == target) {
			return true
		}
	}
	return false
}

fn index_of_eq[T: comparable](xs: T[], target: T) -> Optional[int] {
	let i: int = 0
	for (x in xs) {
		if (x == target) {
			return Some(i)
		}
		i = i + 1
	}
	return None
}

fn main() -> int {
	let xs: int[] = [3, 7, 9]
	print(to_string(contains_eq(xs, 7)))
	print(to_string(contains_eq(xs, 8)))
	let ws: string[] = ["a", "b"]
	print(to_string(contains_eq(ws, "b")))
	print(to_string(unwrap_or(index_of_eq(xs, 9), -1)))
	print(to_string(is_none(index_of_eq(xs, 100))))
	return 0
}
