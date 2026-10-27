// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_pool_resize_test

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
)

type BatchPoolResizeAction struct{}

func TestAccBatchPoolResizeAction_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_batch_pool_resize", "test")
	a := BatchPoolResizeAction{}

	data.ResourceTest(t, a, []acceptance.TestStep{
		{Config: a.basic(data)},
	})
}

func (BatchPoolResizeAction) basic(data acceptance.TestData) string {
	return ""
}
