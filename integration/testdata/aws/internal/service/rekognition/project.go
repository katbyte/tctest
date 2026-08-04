// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rekognition

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_rekognition_project", name="Project")
func newProjectResource(_ context.Context) (framework.ResourceWithConfigure, error) {
	r := &projectResource{}

	return r, nil
}

type projectResource struct {
	framework.ResourceWithModel[projectResourceModel]
}

func (r *projectResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrARN: framework.ARNAttributeComputedOnly(),
			names.AttrName: schema.StringAttribute{
				Required: true,
			},
			names.AttrID: framework.IDAttribute(),
		},
	}
}

func (r *projectResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data projectResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().RekognitionClient(ctx)

	input := &rekognition.CreateProjectInput{
		ProjectName: fwflex.StringFromFramework(ctx, data.Name),
	}

	if _, err := conn.CreateProject(ctx, input); err != nil {
		response.Diagnostics.AddError("creating Rekognition Project", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

type projectResourceModel struct {
	ARN  types.String `tfsdk:"arn"`
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}
