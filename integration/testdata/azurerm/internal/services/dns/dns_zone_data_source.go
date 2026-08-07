// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2018-05-01/zones"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type DnsZoneDataResourceModel struct {
	Name               string   `tfschema:"name"`
	ResourceGroupName  string   `tfschema:"resource_group_name"`
	NumberOfRecordSets int64    `tfschema:"number_of_record_sets"`
	MaxNumberOfRecords int64    `tfschema:"max_number_of_record_sets"`
	NameServers        []string `tfschema:"name_servers"`
}

var _ sdk.DataSource = DnsZoneDataResource{}

type DnsZoneDataResource struct{}

func (DnsZoneDataResource) ModelObject() interface{} {
	return &DnsZoneDataResourceModel{}
}

func (d DnsZoneDataResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return zones.ValidateDnsZoneID
}

func (DnsZoneDataResource) ResourceType() string {
	return "azurerm_dns_zone"
}

func (DnsZoneDataResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},

		"resource_group_name": {
			Type:     pluginsdk.TypeString,
			Optional: true,
			Computed: true,
		},
	}
}

func (DnsZoneDataResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"number_of_record_sets": {
			Type:     pluginsdk.TypeInt,
			Computed: true,
		},

		"max_number_of_record_sets": {
			Type:     pluginsdk.TypeInt,
			Computed: true,
		},

		"name_servers": {
			Type:     pluginsdk.TypeSet,
			Computed: true,
			Elem: &pluginsdk.Schema{
				Type: pluginsdk.TypeString,
			},
		},
	}
}

func (DnsZoneDataResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Dns.Zones

			var state DnsZoneDataResourceModel
			if err := metadata.Decode(&state); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			id := zones.NewDnsZoneID(metadata.Client.Account.SubscriptionId, state.ResourceGroupName, state.Name)

			resp, err := client.Get(ctx, id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return fmt.Errorf("%s was not found", id)
				}
				return fmt.Errorf("retrieving %s: %+v", id, err)
			}

			if model := resp.Model; model != nil {
				if props := model.Properties; props != nil {
					state.NumberOfRecordSets = pointer.From(props.NumberOfRecordSets)
					state.MaxNumberOfRecords = pointer.From(props.MaxNumberOfRecordSets)
					state.NameServers = pointer.From(props.NameServers)
				}
			}

			metadata.SetID(id)
			return metadata.Encode(&state)
		},
	}
}
