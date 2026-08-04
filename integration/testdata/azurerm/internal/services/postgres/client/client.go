// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"

	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/databases"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/servers"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/virtualendpoints"
	"github.com/hashicorp/terraform-provider-azurerm/internal/common"
)

type Client struct {
	FlexibleServersClient          *servers.ServersClient
	FlexibleServerDatabasesClient  *databases.DatabasesClient
	FlexibleServerVirtualEndpoints *virtualendpoints.VirtualEndpointsClient
}

func NewClient(o *common.ClientOptions) (*Client, error) {
	flexibleServersClient, err := servers.NewServersClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building FlexibleServers client: %+v", err)
	}
	o.Configure(flexibleServersClient.Client, o.Authorizers.ResourceManager)

	flexibleServerDatabasesClient, err := databases.NewDatabasesClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building FlexibleServerDatabases client: %+v", err)
	}
	o.Configure(flexibleServerDatabasesClient.Client, o.Authorizers.ResourceManager)

	virtualEndpointsClient, err := virtualendpoints.NewVirtualEndpointsClientWithBaseURI(o.Environment.ResourceManager)
	if err != nil {
		return nil, fmt.Errorf("building VirtualEndpoints client: %+v", err)
	}
	o.Configure(virtualEndpointsClient.Client, o.Authorizers.ResourceManager)

	return &Client{
		FlexibleServersClient:          flexibleServersClient,
		FlexibleServerDatabasesClient:  flexibleServerDatabasesClient,
		FlexibleServerVirtualEndpoints: virtualEndpointsClient,
	}, nil
}
