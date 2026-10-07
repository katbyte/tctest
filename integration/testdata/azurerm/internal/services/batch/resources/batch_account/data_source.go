// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account

import (
	"time"

	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

type BatchAccountDataSource struct{}

var _ sdk.DataSource = BatchAccountDataSource{}

func (r BatchAccountDataSource) ResourceType() string {
	return "azurerm_batch_account"
}

func (r BatchAccountDataSource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{Timeout: 5 * time.Minute}
}
