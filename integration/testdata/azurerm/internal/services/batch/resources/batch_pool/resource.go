// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_pool

import (
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

type BatchPoolResource struct{}

var _ sdk.Resource = BatchPoolResource{}

func (r BatchPoolResource) ResourceType() string {
	return "azurerm_batch_pool"
}
