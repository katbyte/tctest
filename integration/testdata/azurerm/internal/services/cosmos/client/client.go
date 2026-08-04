// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"

	"github.com/hashicorp/go-azure-sdk/resource-manager/cosmosdb/2024-08-15/cosmosdb"
	"github.com/hashicorp/terraform-provider-azurerm/internal/common"
)

type Client struct {
	CosmosDBClient *cosmosdb.CosmosDBClient
}

func NewClient(o *common.ClientOptions) (*Client, error) {
	cosmosdbClient, err := cosmosdb.NewCosmosDBClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building CosmosDB client: %+v", err)
	}
	o.Configure(cosmosdbClient.Client, o.Authorizers.ResourceManager)

	return &Client{
		CosmosDBClient: cosmosdbClient,
	}, nil
}
