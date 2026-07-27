// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package migration

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-provider-azurerm/internal/services/postgres/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ pluginsdk.StateUpgrade = PostgresqlAADAdministratorV0ToV1{}

type PostgresqlAADAdministratorV0ToV1 struct{}

func (PostgresqlAADAdministratorV0ToV1) Schema() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"server_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"resource_group_name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"object_id": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},
	}
}

func (PostgresqlAADAdministratorV0ToV1) UpgradeFunc() pluginsdk.StateUpgraderFunc {
	return func(ctx context.Context, rawState map[string]interface{}, meta interface{}) (map[string]interface{}, error) {
		// old:
		//  /subscriptions/.../resourcegroups/.../providers/Microsoft.DBforPostgreSQL/servers/server1/administrators/activeDirectory
		// new:
		//  /subscriptions/.../resourceGroups/.../providers/Microsoft.DBforPostgreSQL/servers/server1/administrators/objectId
		oldId := rawState["id"].(string)
		id, err := parse.AzureActiveDirectoryAdministratorID(oldId)
		if err != nil {
			return rawState, err
		}

		newId := id.ID()
		log.Printf("[DEBUG] Updating ID from %q to %q", oldId, newId)
		rawState["id"] = newId

		return rawState, nil
	}
}
