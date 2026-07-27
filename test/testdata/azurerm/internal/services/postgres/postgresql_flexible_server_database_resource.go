// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package postgres

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/databases"
	"github.com/hashicorp/go-azure-sdk/resource-manager/postgresqlflexibleservers/2024-08-01/servers"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/postgres/validate"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
)

func resourcePostgresqlFlexibleServerDatabase() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourcePostgresqlFlexibleServerDatabaseCreate,
		Read:   resourcePostgresqlFlexibleServerDatabaseRead,
		Delete: resourcePostgresqlFlexibleServerDatabaseDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := databases.ParseDatabaseID(id)
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},

			"server_id": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: servers.ValidateFlexibleServerID,
			},

			"charset": {
				Type:         pluginsdk.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      "UTF8",
				ValidateFunc: validate.DatabaseCharset,
			},

			"collation": {
				Type:     pluginsdk.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "en_US.utf8",
			},
		},
	}
}

func resourcePostgresqlFlexibleServerDatabaseCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Postgres.FlexibleServerDatabasesClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	serverId, err := servers.ParseFlexibleServerID(d.Get("server_id").(string))
	if err != nil {
		return err
	}

	id := databases.NewDatabaseID(serverId.SubscriptionId, serverId.ResourceGroupName, serverId.FlexibleServerName, d.Get("name").(string))

	payload := databases.Database{
		Properties: &databases.DatabaseProperties{
			Charset:   pointer.To(d.Get("charset").(string)),
			Collation: pointer.To(d.Get("collation").(string)),
		},
	}

	if err := client.CreateThenPoll(ctx, id, payload); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	d.SetId(id.ID())
	return resourcePostgresqlFlexibleServerDatabaseRead(d, meta)
}

func resourcePostgresqlFlexibleServerDatabaseRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Postgres.FlexibleServerDatabasesClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := databases.ParseDatabaseID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	d.Set("name", id.DatabaseName)
	if model := resp.Model; model != nil {
		if props := model.Properties; props != nil {
			d.Set("charset", pointer.From(props.Charset))
			d.Set("collation", pointer.From(props.Collation))
		}
	}

	return nil
}

func resourcePostgresqlFlexibleServerDatabaseDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Postgres.FlexibleServerDatabasesClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := databases.ParseDatabaseID(d.Id())
	if err != nil {
		return err
	}

	if err := client.DeleteThenPoll(ctx, *id); err != nil {
		return fmt.Errorf("deleting %s: %+v", *id, err)
	}

	return nil
}
