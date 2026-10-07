// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_account_test

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
)

type BatchAccountResource struct{}

func TestAccBatchAccount_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_batch_account", "test")
	r := BatchAccountResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data)},
		data.ImportStep(),
	})
}

func TestAccBatchAccount_update(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_batch_account", "test")
	r := BatchAccountResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data)},
		data.ImportStep(),
	})
}

func (BatchAccountResource) basic(data acceptance.TestData) string {
	return ""
}
