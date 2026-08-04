// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parse

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-azure-helpers/resourcemanager/resourceids"
)

type AzureActiveDirectoryAdministratorId struct {
	SubscriptionId string
	ResourceGroup  string
	ServerName     string
	ObjectId       string
}

func NewAzureActiveDirectoryAdministratorID(subscriptionId, resourceGroup, serverName, objectId string) AzureActiveDirectoryAdministratorId {
	return AzureActiveDirectoryAdministratorId{
		SubscriptionId: subscriptionId,
		ResourceGroup:  resourceGroup,
		ServerName:     serverName,
		ObjectId:       objectId,
	}
}

func (id AzureActiveDirectoryAdministratorId) ID() string {
	fmtString := "/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DBforPostgreSQL/servers/%s/administrators/%s"
	return fmt.Sprintf(fmtString, id.SubscriptionId, id.ResourceGroup, id.ServerName, id.ObjectId)
}

func (id AzureActiveDirectoryAdministratorId) String() string {
	segments := []string{
		fmt.Sprintf("Object %q", id.ObjectId),
		fmt.Sprintf("Server Name %q", id.ServerName),
		fmt.Sprintf("Resource Group %q", id.ResourceGroup),
	}
	segmentsStr := strings.Join(segments, " / ")
	return fmt.Sprintf("%s: (%s)", "Azure Active Directory Administrator", segmentsStr)
}

// AzureActiveDirectoryAdministratorID parses a AzureActiveDirectoryAdministrator ID into an AzureActiveDirectoryAdministratorId struct
func AzureActiveDirectoryAdministratorID(input string) (*AzureActiveDirectoryAdministratorId, error) {
	id, err := resourceids.ParseAzureResourceID(input)
	if err != nil {
		return nil, fmt.Errorf("parsing %q as an AzureActiveDirectoryAdministrator ID: %+v", input, err)
	}

	resourceId := AzureActiveDirectoryAdministratorId{
		SubscriptionId: id.SubscriptionID,
		ResourceGroup:  id.ResourceGroup,
	}

	if resourceId.SubscriptionId == "" {
		return nil, fmt.Errorf("ID was missing the 'subscriptions' element")
	}

	if resourceId.ServerName, err = id.PopSegment("servers"); err != nil {
		return nil, err
	}

	if resourceId.ObjectId, err = id.PopSegment("administrators"); err != nil {
		return nil, err
	}

	return &resourceId, nil
}
