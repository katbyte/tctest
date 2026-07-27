// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
)

func TestAccAzureRMDNSZoneDataSource_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_dns_zone", "test")

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: testAccDataSourceDnsZone_basic(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("name_servers.#").Exists(),
			),
		},
	})
}

func testAccDataSourceDnsZone_basic(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "test" {
  name     = "acctestRG-%d"
  location = "%s"
}

resource "azurerm_dns_zone" "test" {
  name                = "acctestzone%d.com"
  resource_group_name = azurerm_resource_group.test.name
}

data "azurerm_dns_zone" "test" {
  name                = azurerm_dns_zone.test.name
  resource_group_name = azurerm_resource_group.test.name
}
`, data.RandomInteger, data.Locations.Primary, data.RandomInteger)
}
