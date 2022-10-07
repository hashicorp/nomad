# Provision a Nomad cluster on AWS

## Pre-requisites

To get started, create the following:

- AWS account
- [API access keys](http://aws.amazon.com/developers/access-keys/)
- [SSH key pair](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)

## Set the AWS environment variables

```bash
$ export AWS_ACCESS_KEY_ID=[AWS_ACCESS_KEY_ID]
$ export AWS_SECRET_ACCESS_KEY=[AWS_SECRET_ACCESS_KEY]
```

## Build an AWS machine image with Packer

[Packer](https://www.packer.io/intro/index.html) is HashiCorp's open source tool 
for creating identical machine images for multiple platforms from a single 
source configuration. The Terraform templates included in this repo reference a 
publicly available Amazon machine image (AMI) by default. The AMI can be customized 
through modifications to the [build configuration script](../shared/scripts/setup.sh) 
and [packer.json](packer.json).


The packer step has been updated to use HCP packer and an HCL template as opposed to the JSON.
In order to use HCP packer, set the following environment variables:
export HCP_ORGANIZATION_ID=

```bash
export HCP_ORGANIZATION=[YOUR_HCP_ORGANIZATION]
export HCP_CLIENT_SECRET=[YOUR_CLIENT_SECRET]
export HCP_CLIENT_ID=[YOUR_CLIENT_IDx
export HCP_PROJECT_ID

```


If you aren't using HCP packer, use the following command to build the AMI:

```bash
$ packer build packer.json
```
Then, set the value of 'USE_HCP_PACKER' to false (it defaults to true) as well as the 'ami' variable to indicate the ami you just built.



=

## Provision a cluster with Terraform

`cd` to an environment subdirectory:

```bash
$ cd env/us-east
```

Update `terraform.tfvars` with your SSH key name and your AMI ID if you created 
a custom AMI:

```bash
region                  = "us-east-1"
ami                     = "ami-09730698a875f6abd"
instance_type           = "t2.medium"
key_name                = "KEY_NAME"
server_count            = "3"
client_count            = "4"
```

Modify the `region`, `instance_type`, `server_count`, and `client_count` variables
as appropriate. At least one client and one server are required. You can 
optionally replace the Nomad binary at runtime by adding the `nomad_binary` 
variable like so:

```bash
region                  = "us-east-1"
ami                     = "ami-09730698a875f6abd"
instance_type           = "t2.medium"
key_name                = "KEY_NAME"
server_count            = "3"
client_count            = "4"
nomad_binary            = "https://releases.hashicorp.com/nomad/0.7.0/nomad_0.7.0_linux_amd64.zip"
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
