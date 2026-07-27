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
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_rekognition_stream_processor", name="Stream Processor")
// @Tags(identifierAttribute="stream_processor_arn")
func newStreamProcessorResource(_ context.Context) (framework.ResourceWithConfigure, error) {
	r := &streamProcessorResource{}

	return r, nil
}

type streamProcessorResource struct {
	framework.ResourceWithModel[streamProcessorResourceModel]
}

func (r *streamProcessorResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			names.AttrName: schema.StringAttribute{
				Required: true,
			},
			names.AttrRoleARN: schema.StringAttribute{
				Required: true,
			},
			"stream_processor_arn": framework.ARNAttributeComputedOnly(),
			names.AttrID:           framework.IDAttribute(),
			names.AttrTags:         tftags.TagsAttribute(),
			names.AttrTagsAll:      tftags.TagsAttributeComputedOnly(),
		},
	}
}

func (r *streamProcessorResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data streamProcessorResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().RekognitionClient(ctx)

	input := &rekognition.CreateStreamProcessorInput{
		Name: fwflex.StringFromFramework(ctx, data.Name),
	}

	if _, err := conn.CreateStreamProcessor(ctx, input); err != nil {
		response.Diagnostics.AddError("creating Rekognition Stream Processor", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

type streamProcessorResourceModel struct {
	ARN     types.String `tfsdk:"stream_processor_arn"`
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	RoleARN types.String `tfsdk:"role_arn"`
	Tags    tftags.Map   `tfsdk:"tags"`
	TagsAll tftags.Map   `tfsdk:"tags_all"`
}
