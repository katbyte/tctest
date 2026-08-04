// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package migration

import (
	"context"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
)

var _ pluginsdk.StateUpgrade = PostgresqlFlexibleServerVirtualEndpointV0ToV1{}

type PostgresqlFlexibleServerVirtualEndpointV0ToV1 struct{}

func (PostgresqlFlexibleServerVirtualEndpointV0ToV1) Schema() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ForceNew: true,
		},

		"source_server_id": {
			Type:     pluginsdk.TypeString,
			Required: true,
		},
	}
}

func (PostgresqlFlexibleServerVirtualEndpointV0ToV1) UpgradeFunc() pluginsdk.StateUpgraderFunc {
	return func(ctx context.Context, rawState map[string]interface{}, meta interface{}) (map[string]interface{}, error) {
		rawState["id"] = rawState["id"].(string)
		return rawState, nil
	}
}
