package cli

import (
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{" YES \n", true},
		{"yes", true}, // EOF without a trailing newline
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"", false}, // EOF, e.g. stdin is empty or /dev/null
		{"yep\n", false},
		{"n\nyes\n", false}, // only the first line is the answer
	}

	for _, tc := range cases {
		got, err := confirm(strings.NewReader(tc.in), "")
		if err != nil {
			t.Fatalf("confirm(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("confirm(%q) = %t, want %t", tc.in, got, tc.want)
		}
	}
}
