// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rekognition

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func streamProcessorSchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func upgradeStreamProcessorStateFromV0(ctx context.Context, request resource.UpgradeStateRequest, response *resource.UpgradeStateResponse) {
	type streamProcessorResourceModelV0 struct {
		ID   types.String `tfsdk:"id"`
		Name types.String `tfsdk:"name"`
	}

	var dataV0 streamProcessorResourceModelV0
	response.Diagnostics.Append(request.State.Get(ctx, &dataV0)...)
	if response.Diagnostics.HasError() {
		return
	}

	dataV1 := streamProcessorResourceModel{
		ID:   dataV0.ID,
		Name: dataV0.Name,
	}

	response.Diagnostics.Append(response.State.Set(ctx, &dataV1)...)
}
