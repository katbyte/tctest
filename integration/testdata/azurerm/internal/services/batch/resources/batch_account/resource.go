// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account

import (
	"time"

	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/batch/resources/batch_account/migration"
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

func (r BatchAccountResource) StateUpgraders() []interface{} {
	return []interface{}{migration.AccountV0ToV1{}, migration.AccountV1ToV2{}}
}
