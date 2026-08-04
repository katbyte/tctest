// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validate

import (
	"fmt"
	"strings"
)

func DatabaseCharset(i interface{}, k string) (warnings []string, errors []error) {
	v, ok := i.(string)
	if !ok {
		errors = append(errors, fmt.Errorf("expected type of %q to be string", k))
		return
	}

	// Postgres charset names are case-insensitive and may contain underscores
	validCharsets := []string{
		"BIG5", "EUC_CN", "LATIN1", "SQL_ASCII", "UTF8", "WIN1252",
	}

	for _, charset := range validCharsets {
		if strings.EqualFold(v, charset) {
			return
		}
	}

	errors = append(errors, fmt.Errorf("%q is not a valid charset for %q", v, k))
	return
}
