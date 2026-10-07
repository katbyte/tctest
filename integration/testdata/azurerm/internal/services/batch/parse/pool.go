// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parse

import (
	"fmt"
	"strings"
)

type PoolId struct {
	AccountName string
	Name        string
}

// PoolID parses "<account>/<pool>" into a PoolId struct
func PoolID(input string) (*PoolId, error) {
	account, name, found := strings.Cut(input, "/")
	if !found {
		return nil, fmt.Errorf("parsing %q as a Pool ID", input)
	}

	return &PoolId{AccountName: account, Name: name}, nil
}
