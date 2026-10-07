// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validate

import (
	"fmt"
	"regexp"
)

func AccountName(i interface{}, k string) (warnings []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}

	if !regexp.MustCompile("^[a-z0-9]{3,24}$").MatchString(v) {
		errors = append(errors, fmt.Errorf("%q must be 3 to 24 lowercase letters and numbers", k))
	}
	return
}
