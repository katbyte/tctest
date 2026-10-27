// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account_test

import (
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
)

// template is config the folder's tests build on; a file like this holds no tests of its own
func (BatchAccountResource) template(data acceptance.TestData) string {
	return ""
}
