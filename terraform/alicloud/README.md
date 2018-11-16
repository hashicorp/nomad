# Provision a Nomad cluster on Alicloud

## Pre-requisites

To get started, create the following:

- Alicloud account
- [API access keys](https://www.alibabacloud.com/help/doc-detail/29009.htm)

## Set the Alicloud environment variables

Set up service endpoint of your region to prevent TLS timeout error.

[Service endpoint in your region](https://www.alibabacloud.com/help/doc-detail/29008.html?spm=a2c5t.11065259.1996646101.searchclickresult.76165207ngmXar)
```bash
$ export ECS_ENDPOINT=[Service endpoint(ecs.us-east-1.aliyuncs.com)] 
$ export TF_VAR_access_key=[ALICLOUD_ACCESS_KEY]
$ export TF_VAR_secret_key=[ALICLOUD_SECRET_KEY]
```

The reason I don't use ALICLOUD_ACCESS_KEY and ALICLOUD_SECRET_KEY is because access_key and secret_key are used in retry_join local variable and you cannot simply access the two ALICLOUD environment variables in the configuration.

The role that holds the key must have the full access to ecs, ram, vpc, eip and kms.

## Build an Alicloud machine image with Packer

Before building a image, you may have not signed up for the alicloud snapshot service first.

[Packer](https://www.packer.io/intro/index.html) is HashiCorp's open source tool 
for creating identical machine images for multiple platforms from a single 
source configuration. The Terraform templates included in this repo reference a 
publicly available Alicloud machine image by default. The  Alicloud machine image can be customized 
through modifications to the [build configuration script](../shared/scripts/setup.sh) 
and [packer.json](packer.json).

Use the following command to build the image:

```bash
$ packer build packer.json
```

## Provision a cluster with Terraform

`cd` to an environment subdirectory:

```bash
$ cd env/us-east
```

Update `terraform.tfvars` with your your Alicloud image ID if you created 
a custom Alicloud image with packer:

```bash
region            = "us-east-1"
image_id          = "ubuntu_16_0402_64_20G_alibase_20180409.vhd"
instance_type     = "ecs.n4.large"
server_count      = "3"
client_count      = "4"
zone              = "us-east-1a"
```

Modify the `region`, `instance_type`, `server_count`, and `client_count` variables
as appropriate. At least one client and one server are required. You can 
optionally replace the Nomad binary at runtime by adding the `nomad_binary` 
variable like so:

```bash
region                  = "us-east-1"
image_id                = "ubuntu_16_0402_64_20G_alibase_20180409.vhd"
instance_type           = "ecs.n4.large"
server_count            = "3"
client_count            = "4"
nomad_binary            = "https://releases.hashicorp.com/nomad/0.8.6/nomad_0.8.6_linux_amd64.zip"
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
$ ssh -i /path/to/private/key ubuntu@PUBLIC_IP
```

The infrastructure that is provisioned for this test environment is configured to 
allow all traffic over port 22. This is obviously not recommended for production 
deployments.

## Next Steps

Click [here](../README.md#test) for next steps.
