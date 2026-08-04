// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres

import (
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

type Registration struct{}

var (
	_ sdk.TypedServiceRegistration   = Registration{}
	_ sdk.UntypedServiceRegistration = Registration{}
)

// Name is the name of this Service
func (r Registration) Name() string {
	return "Postgres"
}

// WebsiteCategories returns a list of categories which can be used for the sidebar
func (r Registration) WebsiteCategories() []string {
	return []string{
		"Database",
	}
}

// SupportedDataSources returns the supported Data Sources supported by this Service
func (r Registration) SupportedDataSources() map[string]*pluginsdk.Resource {
	return map[string]*pluginsdk.Resource{
		"azurerm_postgresql_flexible_server": dataSourcePostgresqlFlexibleServer(),
	}
}

// SupportedResources returns the supported Resources supported by this Service
func (r Registration) SupportedResources() map[string]*pluginsdk.Resource {
	return map[string]*pluginsdk.Resource{
		"azurerm_postgresql_active_directory_administrator": resourcePostgreSQLAdministrator(),
		"azurerm_postgresql_flexible_server":                resourcePostgresqlFlexibleServer(),
		"azurerm_postgresql_flexible_server_database":       resourcePostgresqlFlexibleServerDatabase(),
	}
}

// DataSources returns a list of Data Sources supported by this Service
func (r Registration) DataSources() []sdk.DataSource {
	return []sdk.DataSource{}
}

// Resources returns a list of Resources supported by this Service
func (r Registration) Resources() []sdk.Resource {
	return []sdk.Resource{
		PostgresqlFlexibleServerVirtualEndpointResource{},
	}
}
