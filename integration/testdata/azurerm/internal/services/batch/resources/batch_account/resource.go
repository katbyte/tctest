// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account

import (
	"time"

	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

type BatchAccountResource struct{}

var _ sdk.ResourceWithUpdate = BatchAccountResource{}

func (r BatchAccountResource) ResourceType() string {
	return "azurerm_batch_account"
}

func (r BatchAccountResource) ModelObject() interface{} {
	return &BatchAccountModel{}
}

func (r BatchAccountResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{Timeout: 5 * time.Minute}
}
