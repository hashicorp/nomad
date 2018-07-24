# Provision a Nomad cluster on Azure

## Pre-requisites

To get started, you will need to [create an Azure account](https://azure.microsoft.com/en-us/free/).

## Install the Azure CLI

Run the following commands to install the Azure CLI. Note that you can use the 
[Vagrant](../Vagrantfile) included in this repository to bootstrap a staging 
environment that pre-installs the Azure CLI.

```bash
$ echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ wheezy main" | /
  sudo tee /etc/apt/sources.list.d/azure-cli.list
$ sudo apt-key adv --keyserver packages.microsoft.com --recv-keys 417A0893
$ sudo apt-get install apt-transport-https
$ sudo apt-get update && sudo apt-get install azure-cli
```

## Login to Azure

Use the `az login` CLI command to log in to Azure:

```bash
$ az login

[
  {
    "cloudName": "AzureCloud",
    "id": "SUBSCRIPTION_ID",
    "isDefault": true,
    "name": "Free Trial",
    "state": "Enabled",
    "tenantId": "TENANT_ID",
    "user": {
      "name": "rob@hashicorp.com",
      "type": "user"
    }
  }
]
```

After completing the login process, take note of the values for `id` and 
`tenantId` in the output above. These will be used to set the 
`ARM_SUBSCRIPTION_ID` and `ARM_TENANT_ID` environment variables for Packer 
and Terraform.

ie.
```bash
export ARM_SUBSCRIPTION_ID=SUBSCRIPTION_ID
export ARM_TENANT_ID=TENANT_ID
```


## Create an Application Id and Password

Run the following CLI command to create an application Id and password:

```bash
$ az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/${ARM_SUBSCRIPTION_ID}"

{
  "appId": "CLIENT_ID",
  "displayName": "azure-cli-...",
  "name": "http://azure-cli-...",
  "password": "CLIENT_SECRET",
  "tenant": "TENANT_ID"
}
```

The values for `appId` and `password` above will be used for the `ARM_CLIENT_ID` 
and `ARM_CLIENT_SECRET` environment variables.

ie.
```bash
export ARM_CLIENT_ID=CLIENT_ID
export ARM_CLIENT_SECRET=CLIENT_SECRET
```

## Create an Azure Resource Group

Use the following command to create an Azure [resource group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/xplat-cli-azure-resource-manager#create-a-resource-group) for Packer:

```bash
$ az group create --name packer --location "East US"
```

## Set the Azure Environment Variables

Upto this point we already have set:

```bash
export ARM_SUBSCRIPTION_ID=[ARM_SUBSCRIPTION_ID]  
export ARM_TENANT_ID=[ARM_TENANT_ID]  
export ARM_CLIENT_ID=[ARM_CLIENT_ID]  
export ARM_CLIENT_SECRET=[ARM_CLIENT_SECRET]  
```

We need to add a new one:
```bash
export AZURE_RESOURCE_GROUP=packer  
```

## Build an Azure machine image with Packer

[Packer](https://www.packer.io/intro/index.html) is HashiCorp's open source tool 
for creating identical machine images for multiple platforms from a single 
source configuration. The machine image created here can be customized through 
modifications to the [build configuration file](packer.json) and the 
[shell script](../shared/scripts/setup.sh).

Use the following command to build the machine image:

```bash
$ packer build packer.json
```

After the Packer build process completes, you can retrieve the image Id using the 
following CLI command:

```bash
$ az image list --query "[?tags.Product=='Hashistack'].id"

[
  "/subscriptions/SUBSCRIPTION_ID/resourceGroups/PACKER/providers/Microsoft.Compute/images/hashistack"
]
```

The following CLI command can be used to delete the image if necessary:

```bash
$ az image delete --name hashistack --resource-group packer
```

## Provision a cluster with Terraform

`cd` to an environment subdirectory:

```bash
$ cd env/EastUS
```

Consul supports a cloud-based auto join feature which includes support for Azure. 
The feature requires that we create a service principal with the `Reader` role. 
Run the following command to create an Azure service principal for Consul auto join: 

```bash
$ az ad sp create-for-rbac --role="Reader" --scopes="/subscriptions/[SUBSCRIPTION_ID]"

{
  "appId": "CLIENT_ID",
  "displayName": "azure-cli-...",
  "name": "http://azure-cli-...",
  "password": "CLIENT_SECRET",
  "tenant": "TENANT_ID"
}
```

Update `terraform.tfvars` with you SUBSCRIPTION_ID, TENANT_ID, CLIENT_ID and CLIENT_SECRET. Use the CLIENT_ID and CLIENT_SECRET created above for the service principal:

```bash
location = "East US"
image_id = "/subscriptions/SUBSCRIPTION_ID/resourceGroups/PACKER/providers/Microsoft.Compute/images/hashistack"
vm_size = "Standard_DS1_v2"
server_count = 1
client_count = 4
retry_join = "provider=azure tag_name=ConsulAutoJoin tag_value=auto-join subscription_id=SUBSCRIPTION_ID tenant_id=TENANT_ID client_id=CLIENT_ID secret_access_key=CLIENT_SECRET"
```

Provision the cluster:

```bash
$ terraform init
$ terraform get
$ terraform plan
$ terraform apply
```

## Access the cluster

SSH to one of the servers using its public IP:

```bash
$ ssh -i azure-hashistack.pem ubuntu@PUBLIC_IP
```

`azure-hashistack.pem` above is auto-created during the provisioning process. The 
infrastructure that is provisioned for this test environment is configured to 
allow all traffic over port 22. This is obviously not recommended for production 
deployments.

## Next steps

Click [here](../README.md#test) for next steps.
