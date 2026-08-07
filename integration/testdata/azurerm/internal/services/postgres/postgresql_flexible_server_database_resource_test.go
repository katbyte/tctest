// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/databases"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

type PostgresqlFlexibleServerDatabaseResource struct{}

func TestAccPostgresqlFlexibleServerDatabase_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_postgresql_flexible_server_database", "test")
	r := PostgresqlFlexibleServerDatabaseResource{}

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

func TestAccPostgresqlFlexibleServerDatabase_charset(t *testing.T) {
	data := acceptance.BuildTestData(t, "azurerm_postgresql_flexible_server_database", "test")
	r := PostgresqlFlexibleServerDatabaseResource{}

	data.ResourceTest(t, r, []acceptance.TestStep{
		{
			Config: r.charset(data, "SQL_ASCII"),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).ExistsInAzure(r),
				check.That(data.ResourceName).Key("charset").HasValue("SQL_ASCII"),
			),
		},
		data.ImportStep(),
	})
}

func (PostgresqlFlexibleServerDatabaseResource) Exists(ctx context.Context, clients *clients.Client, state *pluginsdk.InstanceState) (*bool, error) {
	id, err := databases.ParseDatabaseID(state.ID)
	if err != nil {
		return nil, err
	}

	resp, err := clients.Postgres.FlexibleServerDatabasesClient.Get(ctx, *id)
	if err != nil {
		return nil, fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	return utils.Bool(resp.Model != nil), nil
}

func (PostgresqlFlexibleServerDatabaseResource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

resource "azurerm_postgresql_flexible_server_database" "test" {
  name      = "acctest-fsd-%d"
  server_id = azurerm_postgresql_flexible_server.test.id
}
`, PostgresqlFlexibleServerResource{}.basic(data), data.RandomInteger)
}

func (PostgresqlFlexibleServerDatabaseResource) charset(data acceptance.TestData, charset string) string {
	return fmt.Sprintf(`
%s

resource "azurerm_postgresql_flexible_server_database" "test" {
  name      = "acctest-fsd-%d"
  server_id = azurerm_postgresql_flexible_server.test.id
  charset   = "%s"
  collation = "C"
}
`, PostgresqlFlexibleServerResource{}.basic(data), data.RandomInteger, charset)
}
