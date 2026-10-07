// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch

import (
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/batch/actions/batch_pool_resize"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/batch/resources/batch_account"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/batch/resources/batch_pool"
)

type Registration struct{}

func (r Registration) Name() string {
	return "Batch"
}

func (r Registration) Resources() []sdk.Resource {
	return []sdk.Resource{
		batch_account.BatchAccountResource{},
		batch_pool.BatchPoolResource{},
	}
}

func (r Registration) DataSources() []sdk.DataSource {
	return []sdk.DataSource{
		batch_account.BatchAccountDataSource{},
	}
}

func (r Registration) Actions() []func() action.Action {
	return []func() action.Action{
		batch_pool_resize.Action,
	}
}
