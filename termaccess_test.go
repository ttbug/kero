package main

import "testing"

func TestLastLines(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		n    int
		want string
	}{
		{"empty", "", 24, ""},
		{"blank", "  \n \n", 24, ""},
		{"zero lines", "first\nsecond", 0, ""},
		{"negative lines", "first\nsecond", -1, ""},
		{"last line", "first\nsecond\nthird\n  \n", 1, "third"},
		{"last two lines", "first\nsecond\nthird\n", 2, "second\nthird"},
		{"more lines than text", "first\nsecond", 24, "first\nsecond"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastLines(tt.text, tt.n); got != tt.want {
				t.Errorf("lastLines(%q, %d) = %q; want %q", tt.text, tt.n, got, tt.want)
			}
		})
	}
}
