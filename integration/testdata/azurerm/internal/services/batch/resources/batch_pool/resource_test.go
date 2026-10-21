// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_pool_test

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
)

type BatchPoolResource struct{}

func TestAccBatchPool_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_batch_pool", "test")
	r := BatchPoolResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{Config: r.basic(data)},
	})
}

func (BatchPoolResource) basic(data acceptance.TestData) string {
	return ""
}
