// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import "testing"

func TestNormalizeIPv6Address(t *testing.T) {
	cases := []struct {
		Input    string
		Expected string
	}{
		{"", ""},
		{"2001:0db8:85a3:0:0:8a2e:0370:7334", "2001:db8:85a3::8a2e:370:7334"},
		{"2001:0DB8:85A3:0:0:8A2E:0370:7334", "2001:db8:85a3::8a2e:370:7334"},
		{"not-an-ip", ""},
	}

	for _, tc := range cases {
		actual := NormalizeIPv6Address(tc.Input)
		if actual != tc.Expected {
			t.Fatalf("expected NormalizeIPv6Address(%q) to be %q, got %q", tc.Input, tc.Expected, actual)
		}
	}
}
