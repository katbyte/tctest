// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/virtualendpoints"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type PostgresqlFlexibleServerVirtualEndpointResource struct{}

func TestAccPostgresqlFlexibleServerVirtualEndpoint_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_postgresql_flexible_server_virtual_endpoint", "test")
	r := PostgresqlFlexibleServerVirtualEndpointResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
			),
		},
		data.ImportStep(),
	})
}

func (PostgresqlFlexibleServerVirtualEndpointResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := virtualendpoints.ParseVirtualEndpointID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Postgres.FlexibleServerVirtualEndpoints.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	return utils.Bool(resp.Model != nil), nil
}

func (PostgresqlFlexibleServerVirtualEndpointResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_postgresql_flexible_server" "test_replica" {
  name                = "acctest-fs-replica-%d"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  version             = "16"
  sku_name            = "GP_Standard_D2s_v3"
}

resource "azurerm_postgresql_flexible_server_virtual_endpoint" "test" {
  name              = "acctest-ve-%d"
  source_server_id  = azurerm_postgresql_flexible_server.test.id
  replica_server_id = azurerm_postgresql_flexible_server.test_replica.id
  type              = "ReadWrite"
}
`, PostgresqlFlexibleServerResource{}.basic(data), data.RandomInteger, data.RandomInteger)
}
