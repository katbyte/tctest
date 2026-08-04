// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validate

import "testing"

func TestDatabaseCharset(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{input: "", valid: false},
		{input: "UTF8", valid: true},
		{input: "utf8", valid: true},
		{input: "KOI8R2000", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, errors := DatabaseCharset(tt.input, "charset")
			valid := len(errors) == 0
			if valid != tt.valid {
				t.Errorf("expected valid = %t, got %t", tt.valid, valid)
			}
		})
	}
}
