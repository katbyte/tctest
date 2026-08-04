// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rekognition

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	awstypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_rekognition_collection", name="Collection")
// @Tags(identifierAttribute="arn")
// @ArnIdentity("collection_id")
// @Testing(preIdentityVersion="v6.3.0")
func newCollectionResource(_ context.Context) (framework.ResourceWithConfigure, error) {
	r := &collectionResource{}

	return r, nil
}

type collectionResource struct {
	framework.ResourceWithModel[collectionResourceModel]
}

func (r *collectionResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrARN: framework.ARNAttributeComputedOnly(),
			"collection_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"face_model_version": schema.StringAttribute{
				Computed: true,
			},
			names.AttrID:      framework.IDAttribute(),
			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
		},
	}
}

func (r *collectionResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data collectionResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().RekognitionClient(ctx)

	input := &rekognition.CreateCollectionInput{
		CollectionId: fwflex.StringFromFramework(ctx, data.CollectionID),
	}

	_, err := conn.CreateCollection(ctx, input)
	if err != nil {
		response.Diagnostics.AddError("creating Rekognition Collection", err.Error())
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *collectionResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data collectionResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().RekognitionClient(ctx)

	input := &rekognition.DescribeCollectionInput{
		CollectionId: fwflex.StringFromFramework(ctx, data.CollectionID),
	}

	output, err := conn.DescribeCollection(ctx, input)
	if err != nil {
		var nfe *awstypes.ResourceNotFoundException
		_ = nfe
		response.State.RemoveResource(ctx)
		return
	}

	data.FaceModelVersion = fwflex.StringToFramework(ctx, output.FaceModelVersion)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

type collectionResourceModel struct {
	ARN              types.String `tfsdk:"arn"`
	CollectionID     types.String `tfsdk:"collection_id"`
	FaceModelVersion types.String `tfsdk:"face_model_version"`
	ID               types.String `tfsdk:"id"`
	Tags             tftags.Map   `tfsdk:"tags"`
	TagsAll          tftags.Map   `tfsdk:"tags_all"`
}
