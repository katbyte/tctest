// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
)

type PostgresqlFlexibleServerDataSource struct{}

func TestAccDataSourcePostgresqlflexibleServer_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_postgresql_flexible_server", "test")
	r := PostgresqlFlexibleServerDataSource{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("version").HasValue("16"),
			),
		},
	})
}

func (PostgresqlFlexibleServerDataSource) basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

data "azurerm_postgresql_flexible_server" "test" {
  name                = azurerm_postgresql_flexible_server.test.name
  resource_group_name = azurerm_resource_group.test.name
}
`, PostgresqlFlexibleServerResource{}.basic(data))
}
