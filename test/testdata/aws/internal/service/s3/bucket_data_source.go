// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKDataSource("aws_s3_bucket", name="Bucket")
func dataSourceBucket() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceBucketRead,

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrBucket: {
				Type:     schema.TypeString,
				Required: true,
			},
			names.AttrRegion: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceBucketRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	input := &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}

	if _, err := conn.HeadBucket(ctx, input); err != nil {
		return sdkdiag.AppendErrorf(diags, "reading S3 Bucket (%s): %s", bucket, err)
	}

	d.SetId(bucket)
	d.Set(names.AttrARN, fmt.Sprintf("arn:aws:s3:::%s", bucket))

	return diags
}
