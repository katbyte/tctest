// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_pool_resize

import (
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

type BatchPoolResizeAction struct {
	sdk.ActionMetadata
}

var _ sdk.Action = &BatchPoolResizeAction{}

func Action() action.Action {
	return &BatchPoolResizeAction{}
}
