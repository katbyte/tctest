#!/usr/bin/env bash

export TCTEST_SERVER="ci.katbyte.me"
export TCTEST_TOKEN="abc123"
export TCTEST_REPO="terraform-providers/terraform-provider-azurerm"

# The build type ID, or build configuration ID, can be found in the Teamcity website, in the URL path after "/buildConfiguration/"  https://mycompany.teamcity.com/buildConfiguration/<THIS_IS_THE_ID>#all-projects
export TCTEST_BUILDTYPEID="TCBuildTypeId"
