// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/servers"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/virtualendpoints"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/postgres/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
)

type PostgresqlFlexibleServerVirtualEndpointModel struct {
	Name            string `tfschema:"name"`
	SourceServerId  string `tfschema:"source_server_id"`
	ReplicaServerId string `tfschema:"replica_server_id"`
	Type            string `tfschema:"type"`
}

type PostgresqlFlexibleServerVirtualEndpointResource struct{}

var (
	_ sdk.ResourceWithUpdate         = PostgresqlFlexibleServerVirtualEndpointResource{}
	_ sdk.ResourceWithStateMigration = PostgresqlFlexibleServerVirtualEndpointResource{}
)

func (r PostgresqlFlexibleServerVirtualEndpointResource) ResourceType() string {
	return "azurerm_postgresql_flexible_server_virtual_endpoint"
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) ModelObject() interface{} {
	return &PostgresqlFlexibleServerVirtualEndpointModel{}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) IDValidationFunc() pluginsdk.SchemaValidateFunc {
	return virtualendpoints.ValidateVirtualEndpointID
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) StateUpgraders() sdk.StateUpgradeData {
	return sdk.StateUpgradeData{
		SchemaVersion: 1,
		Upgraders: map[int]pluginsdk.StateUpgrade{
			0: migration.PostgresqlFlexibleServerVirtualEndpointV0ToV1{},
		},
	}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Arguments() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{
		"name": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ForceNew:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},

		"source_server_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: servers.ValidateFlexibleServerID,
		},

		"replica_server_id": {
			Type:         pluginsdk.TypeString,
			Required:     true,
			ValidateFunc: servers.ValidateFlexibleServerID,
		},

		"type": {
			Type:     pluginsdk.TypeString,
			Required: true,
			ValidateFunc: validation.StringInSlice([]string{
				string(virtualendpoints.VirtualEndpointTypeReadWrite),
			}, false),
		},
	}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Attributes() map[string]*pluginsdk.Schema {
	return map[string]*pluginsdk.Schema{}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Create() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Postgres.FlexibleServerVirtualEndpoints

			var model PostgresqlFlexibleServerVirtualEndpointModel
			if err := metadata.Decode(&model); err != nil {
				return fmt.Errorf("decoding: %+v", err)
			}

			sourceId, err := servers.ParseFlexibleServerID(model.SourceServerId)
			if err != nil {
				return err
			}

			id := virtualendpoints.NewVirtualEndpointID(sourceId.SubscriptionId, sourceId.ResourceGroupName, sourceId.FlexibleServerName, model.Name)

			payload := virtualendpoints.VirtualEndpointResource{
				Properties: &virtualendpoints.VirtualEndpointResourceProperties{
					EndpointType: pointer.To(virtualendpoints.VirtualEndpointType(model.Type)),
					Members:      pointer.To([]string{model.ReplicaServerId}),
				},
			}

			if err := client.CreateThenPoll(ctx, id, payload); err != nil {
				return fmt.Errorf("creating %s: %+v", id, err)
			}

			metadata.SetID(id)
			return nil
		},
	}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Read() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 5 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Postgres.FlexibleServerVirtualEndpoints

			id, err := virtualendpoints.ParseVirtualEndpointID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			resp, err := client.Get(ctx, *id)
			if err != nil {
				if response.WasNotFound(resp.HttpResponse) {
					return metadata.MarkAsGone(id)
				}
				return fmt.Errorf("retrieving %s: %+v", *id, err)
			}

			state := PostgresqlFlexibleServerVirtualEndpointModel{
				Name: id.VirtualEndpointName,
			}
			if model := resp.Model; model != nil && model.Properties != nil {
				state.Type = string(pointer.From(model.Properties.EndpointType))
			}

			return metadata.Encode(&state)
		},
	}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Update() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			return nil
		},
	}
}

func (r PostgresqlFlexibleServerVirtualEndpointResource) Delete() sdk.ResourceFunc {
	return sdk.ResourceFunc{
		Timeout: 30 * time.Minute,
		Func: func(ctx context.Context, metadata sdk.ResourceMetaData) error {
			client := metadata.Client.Postgres.FlexibleServerVirtualEndpoints

			id, err := virtualendpoints.ParseVirtualEndpointID(metadata.ResourceData.Id())
			if err != nil {
				return err
			}

			if err := client.DeleteThenPoll(ctx, *id); err != nil {
				return fmt.Errorf("deleting %s: %+v", *id, err)
			}

			return nil
		},
	}
}
