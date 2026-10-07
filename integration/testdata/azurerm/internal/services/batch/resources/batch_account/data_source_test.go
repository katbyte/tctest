// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account_test

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
)

type BatchAccountDataSource struct{}

func TestAccBatchAccountDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_batch_account", "test")
	r := BatchAccountDataSource{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{Config: r.basic(data)},
	})
}

func (BatchAccountDataSource) basic(data acceptance.TestData) string {
	return ""
}
