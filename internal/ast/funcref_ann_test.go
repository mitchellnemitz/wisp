package ast

import "testing"

// TestIsFuncrefAnn pins the shape predicate shared by the array-element
// parenthesization guard and the funcref dispatch branch in formatType.
func TestIsFuncrefAnn(t *testing.T) {
	cases := []struct {
		ann  string
		want bool
	}{
		{"fn(int)->bool", true},
		{"fn()->void", true},
		{"int", false},
		{"Box[int]", false},
		{"{string:int}", false},
		{"MyStruct", false},
	}
	for _, c := range cases {
		if got := isFuncrefAnn(c.ann); got != c.want {
			t.Errorf("isFuncrefAnn(%q) = %v, want %v", c.ann, got, c.want)
		}
	}
}
