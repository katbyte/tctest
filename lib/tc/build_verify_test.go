package tc

import "testing"

func TestXmlEscape(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`Test(A|B)`:            `Test(A|B)`,
		`a&b`:                  `a&amp;b`,
		`<x>`:                  `&lt;x&gt;`,
		`say "hi"`:             `say &#34;hi&#34;`,
		`https://x?a=1&b=2`:    `https://x?a=1&amp;b=2`,
		`(TestAccFoo|TestBar)`: `(TestAccFoo|TestBar)`,
	}
	for in, want := range cases {
		if got := xmlEscape(in); got != want {
			t.Errorf("xmlEscape(%q) = %q, want %q", in, got, want)
		}
	}
}
