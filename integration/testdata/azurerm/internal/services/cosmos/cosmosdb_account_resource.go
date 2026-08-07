// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cosmos

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/pointer"
	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/commonschema"
	"github.com/hashicorp/go-azure-helpers/resourcemanager/location"
	"github.com/hashicorp/go-azure-sdk/resource-manager/cosmosdb/2024-08-15/cosmosdb"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
)

func resourceCosmosDbAccount() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceCosmosDbAccountCreate,
		Read:   resourceCosmosDbAccountRead,
		Delete: resourceCosmosDbAccountDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(3 * time.Hour),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(3 * time.Hour),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := cosmosdb.ParseDatabaseAccountID(id)
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": commonschema.Location(),

			"resource_group_name": commonschema.ResourceGroupName(),

			"offer_type": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(cosmosdb.DatabaseAccountOfferTypeStandard),
				}, false),
			},

			"consistency_policy": {
				Type:     pluginsdk.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &pluginsdk.Resource{
					Schema: map[string]*pluginsdk.Schema{
						"consistency_level": {
							Type:     pluginsdk.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceCosmosDbAccountCreate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Cosmos.CosmosDBClient
	ctx, cancel := timeouts.ForCreate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id := cosmosdb.NewDatabaseAccountID(meta.(*clients.Client).Account.SubscriptionId, d.Get("resource_group_name").(string), d.Get("name").(string))

	existing, err := client.DatabaseAccountsGet(ctx, id)
	if err != nil {
		if !response.WasNotFound(existing.HttpResponse) {
			return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
		}
	}
	if !response.WasNotFound(existing.HttpResponse) {
		return tf.ImportAsExistsError("azurerm_cosmosdb_account", id.ID())
	}

	payload := cosmosdb.DatabaseAccountCreateUpdateParameters{
		Location: pointer.To(location.Normalize(d.Get("location").(string))),
		Properties: cosmosdb.DatabaseAccountCreateUpdateProperties{
			DatabaseAccountOfferType: cosmosdb.DatabaseAccountOfferType(d.Get("offer_type").(string)),
		},
	}

	if err := client.DatabaseAccountsCreateOrUpdateThenPoll(ctx, id, payload); err != nil {
		return fmt.Errorf("creating %s: %+v", id, err)
	}

	d.SetId(id.ID())
	return resourceCosmosDbAccountRead(d, meta)
}

func resourceCosmosDbAccountRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Cosmos.CosmosDBClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := cosmosdb.ParseDatabaseAccountID(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.DatabaseAccountsGet(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving %s: %+v", *id, err)
	}

	d.Set("name", id.DatabaseAccountName)
	d.Set("resource_group_name", id.ResourceGroupName)
	if model := resp.Model; model != nil {
		d.Set("location", location.NormalizeNilable(model.Location))
	}

	return nil
}

func resourceCosmosDbAccountDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Cosmos.CosmosDBClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := cosmosdb.ParseDatabaseAccountID(d.Id())
	if err != nil {
		return err
	}

	if err := client.DatabaseAccountsDeleteThenPoll(ctx, *id); err != nil {
		return fmt.Errorf("deleting %s: %+v", *id, err)
	}

	return nil
}
