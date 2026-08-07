// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"

	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2018-05-01/recordsets"
	"github.com/hashicorp/go-azure-sdk/resource-manager/dns/2018-05-01/zones"
	"github.com/hashicorp/terraform-provider-azurerm/internal/common"
)

type Client struct {
	RecordSets *recordsets.RecordSetsClient
	Zones      *zones.ZonesClient
}

func NewClient(o *common.ClientOptions) (*Client, error) {
	recordSetsClient, err := recordsets.NewRecordSetsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building RecordSets client: %+v", err)
	}
	o.Configure(recordSetsClient.Client, o.Authorizers.ResourceManager)

	zonesClient, err := zones.NewZonesClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building Zones client: %+v", err)
	}
	o.Configure(zonesClient.Client, o.Authorizers.ResourceManager)

	return &Client{
		RecordSets: recordSetsClient,
		Zones:      zonesClient,
	}, nil
}
