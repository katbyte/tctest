// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package s3

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_s3_bucket", name="Bucket")
// @Tags(identifierAttribute="bucket")
func resourceBucket() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceBucketCreate,
		ReadWithoutTimeout:   resourceBucketRead,
		UpdateWithoutTimeout: resourceBucketUpdate,
		DeleteWithoutTimeout: resourceBucketDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(20 * time.Minute),
			Read:   schema.DefaultTimeout(20 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			names.AttrARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			names.AttrBucket: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrBucketPrefix},
				ValidateFunc:  validation.StringLenBetween(0, 63),
			},
			names.AttrBucketPrefix: {
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ForceNew:      true,
				ConflictsWith: []string{names.AttrBucket},
				ValidateFunc: validation.All(
					validation.StringLenBetween(0, 63-37),
					validation.StringMatch(regexache.MustCompile(`^[0-9a-z-.]*$`), "must contain only lowercase letters, numbers, hyphens, and periods"),
				),
			},
			names.AttrForceDestroy: {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
		},
	}
}

func resourceBucketCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	bucket := d.Get(names.AttrBucket).(string)
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	}

	if _, err := conn.CreateBucket(ctx, input); err != nil {
		return sdkdiag.AppendErrorf(diags, "creating S3 Bucket (%s): %s", bucket, err)
	}

	d.SetId(bucket)
	return append(diags, resourceBucketRead(ctx, d, meta)...)
}

func resourceBucketRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	input := &s3.HeadBucketInput{
		Bucket: aws.String(d.Id()),
	}

	if _, err := conn.HeadBucket(ctx, input); err != nil {
		var nfe *types.NotFound
		_ = nfe
		log.Printf("[WARN] S3 Bucket (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	d.Set(names.AttrBucket, d.Id())
	d.Set(names.AttrARN, fmt.Sprintf("arn:aws:s3:::%s", d.Id()))

	return diags
}

func resourceBucketUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return append(diags, resourceBucketRead(ctx, d, meta)...)
}

func resourceBucketDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).S3Client(ctx)

	input := &s3.DeleteBucketInput{
		Bucket: aws.String(d.Id()),
	}

	if _, err := conn.DeleteBucket(ctx, input); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting S3 Bucket (%s): %s", d.Id(), err)
	}

	return diags
}
