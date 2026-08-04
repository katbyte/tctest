// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
)

// @SDKDataSource("aws_s3_buckets", name="Buckets")
func dataSourceBuckets() *schema.Resource {
	return &schema.Resource{
		ReadWithoutTimeout: dataSourceBucketsRead,

		Schema: map[string]*schema.Schema{
			"buckets": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func dataSourceBucketsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	output, err := conn.ListBuckets(ctx, nil)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "listing S3 Buckets: %s", err)
	}

	names := make([]string, 0, len(output.Buckets))
	for _, bucket := range output.Buckets {
		if bucket.Name != nil {
			names = append(names, *bucket.Name)
		}
	}

	d.SetId("buckets")
	d.Set("buckets", names)

	return diags
}
