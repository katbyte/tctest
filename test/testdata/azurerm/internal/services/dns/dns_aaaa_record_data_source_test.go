// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance"
	"github.com/hashicorp/terraform-provider-azurerm/internal/acceptance/check"
)

func TestAccDataSourceDnsAAAARecord_basic(t *testing.T) {
	data := acceptance.BuildTestData(t, "data.azurerm_dns_aaaa_record", "test")
	r := DnsAAAARecordResource{}

	data.DataSourceTest(t, []acceptance.TestStep{
		{
			Config: r.basicDataSource(data),
			Check: acceptance.ComposeTestCheckFunc(
				check.That(data.ResourceName).Key("ttl").Exists(),
			),
		},
	})
}

func (r DnsAAAARecordResource) basicDataSource(data acceptance.TestData) string {
	return fmt.Sprintf(`
%s

data "azurerm_dns_aaaa_record" "test" {
  name                = azurerm_dns_aaaa_record.test.name
  resource_group_name = azurerm_resource_group.test.name
  zone_name           = azurerm_dns_zone.test.name
}
`, r.basic(data))
}
