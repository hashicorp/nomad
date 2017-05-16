# Deploy a Nomad cluster in AWS

Deploys one or more servers running Nomad,  Consul and Vault as well a configurable number of clients.

## Setup

Clone the repo and (optionally) use the included Vagrantfile to bootstrap a local staging environment:

```bash
$ git clone git@github.com:hashicorp/nomad.git
$ cd terraform/aws
$ vagrant up && vagrant ssh
```

### Pre-requisites

You will need the following:

- AWS account
- [API access keys](http://aws.amazon.com/developers/access-keys/)
- [SSH key pair](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)

If you are using a Vagrant environment, you will need to copy your private key to it. If not, you will need to [install Terraform](https://www.terraform.io/intro/getting-started/install.html).

Set environment variables for your AWS credentials:

```bash
$ export AWS_ACCESS_KEY_ID=[ACCESS_KEY_ID]
$ export AWS_SECRET_ACCESS_KEY=[SECRET_ACCESS_KEY]
```

## Provision

`cd` to one of the environment subdirectories:

```bash
$ cd aws/env/us-east
```

Update terraform.tfvars with your SSH key name:

```bash
region                  = "us-east-1"
ami                     = "ami-28a1dd3e"
instance_type           = "t2.medium"
key_name                = "KEY"
key_file                = "/home/vagrant/.ssh/KEY.pem"
server_count            = "3"
client_count            = "4"
```
For example:

```bash
region                  = "us-east-1"
ami                     = "ami-28a1dd3e"
instance_type           = "t2.medium"
key_name                = "hashi-us-east-1"
key_file                = "/home/vagrant/.ssh/hashi-us-east-1.pem"
server_count            = "3"
client_count            = "4"
```

Provision:

```bash
terraform get
terraform plan
terraform apply
```

## Access the cluster

SSH to a server using its public IP. For example:

```bash
$ ssh -i /home/vagrant/.ssh/KEY.pem ubuntu@SERVER_PUBLIC_IP
```

Please note that the AWS security group is configured by default to allow all traffic over port 22. This is not recommended for production deployments.

Optionally, initialize and Unseal Vault:

```bash
$ vault init -key-shares=1 -key-threshold=1
$ vault unseal
$ export VAULT_TOKEN=[INITIAL_ROOT_TOKEN]
```

Test Consul, Nomad:

```bash
$ consul members
$ nomad server-members
$ nomad node-status
```

See the [examples](../examples/README.md).

