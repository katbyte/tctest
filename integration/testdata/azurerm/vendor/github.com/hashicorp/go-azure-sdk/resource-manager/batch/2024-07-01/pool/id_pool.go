// Copyright (c) HashiCorp Inc. All rights reserved.
// Licensed under the MIT License. See NOTICE.txt in the project root for license information.

package pool

import (
	"fmt"
	"strings"
)

type PoolId struct {
	BatchAccountName string
	PoolName         string
}

func ParsePoolID(input string) (*PoolId, error) {
	account, name, found := strings.Cut(input, "/pools/")
	if !found {
		return nil, fmt.Errorf("parsing %q as a Pool ID", input)
	}

	return &PoolId{BatchAccountName: account, PoolName: name}, nil
}
