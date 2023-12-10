# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

location = "East US"
image_id = "/subscriptions/SUBSCRIPTION_ID/resourceGroups/PACKER/providers/Microsoft.Compute/images/hashistack"
vm_size = "Standard_DS1_v2"
server_count = 1
client_count = 4
retry_join = "provider=azure tag_name=ConsulAutoJoin tag_value=auto-join subscription_id=SUBSCRIPTION_ID tenant_id=TENANT_ID client_id=CLIENT_ID secret_access_key=CLIENT_SECRET"
