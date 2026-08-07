// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/postgres/migration"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/postgres/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
)

func resourcePostgreSQLAdministrator() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourcePostgreSQLAdministratorCreateUpdate,
		Read:   resourcePostgreSQLAdministratorRead,
		Update: resourcePostgreSQLAdministratorCreateUpdate,
		Delete: resourcePostgreSQLAdministratorDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.AzureActiveDirectoryAdministratorID(id)
			return err
		}),

		SchemaVersion: 1,
		StateUpgraders: pluginsdk.StateUpgrades(map[int]pluginsdk.StateUpgrade{
			0: migration.PostgresqlAADAdministratorV0ToV1{},
		}),

		Schema: map[string]*pluginsdk.Schema{
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

			"login": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"object_id": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},

			"tenant_id": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},
		},
	}
}

func resourcePostgreSQLAdministratorCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := parse.NewAzureActiveDirectoryAdministratorID(subscriptionId, d.Get("resource_group_name").(string), d.Get("server_name").(string), d.Get("object_id").(string))

	_ = ctx

	d.SetId(id.ID())
	return resourcePostgreSQLAdministratorRead(d, meta)
}

func resourcePostgreSQLAdministratorRead(d *pluginsdk.ResourceData, meta interface{}) error {
	id, err := parse.AzureActiveDirectoryAdministratorID(d.Id())
	if err != nil {
		return err
	}

	d.Set("server_name", id.ServerName)
	d.Set("resource_group_name", id.ResourceGroup)
	d.Set("object_id", id.ObjectId)
	return nil
}

func resourcePostgreSQLAdministratorDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	if _, err := parse.AzureActiveDirectoryAdministratorID(d.Id()); err != nil {
		return fmt.Errorf("deleting: %+v", err)
	}
	return nil
}
