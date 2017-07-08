# Provision a Nomad cluster on AWS with Packer & Terraform

Use this to easily provision a Nomad sandbox environment on AWS with 
[Packer](https://packer.io) and [Terraform](https://terraform.io). 
[Consul](https://www.consul.io/intro/index.html) and 
[Vault](https://www.vaultproject.io/intro/index.html) are also installed 
(colocated for convenience). The intention is to allow easy exploration of 
Nomad and its integrations with the HashiCorp stack. This is *not* meant to be
a production ready environment. A demonstration of [Nomad's Apache Spark 
integration](examples/spark/README.md) is included. 

## Setup

Clone this repo and (optionally) use [Vagrant](https://www.vagrantup.com/intro/index.html) 
to bootstrap a local staging environment:

```bash
$ git clone git@github.com:hashicorp/nomad.git
$ cd terraform/aws
$ vagrant up && vagrant ssh
```

The Vagrant staging environment pre-installs Packer, Terraform, and Docker.

### Pre-requisites

You will need the following:

- AWS account
- [API access keys](http://aws.amazon.com/developers/access-keys/)
- [SSH key pair](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html)

Set environment variables for your AWS credentials:

```bash
$ export AWS_ACCESS_KEY_ID=[ACCESS_KEY_ID]
$ export AWS_SECRET_ACCESS_KEY=[SECRET_ACCESS_KEY]
```

## Provision a cluster

`cd` to an environment subdirectory:

```bash
$ cd env/us-east
```

Update terraform.tfvars with your SSH key name:

```bash
region                  = "us-east-1"
ami                     = "ami-76787e60"
instance_type           = "t2.medium"
key_name                = "KEY_NAME"
server_count            = "3"
client_count            = "4"
```

Note that a pre-provisioned, publicly available AMI is used by default 
(for the `us-east-1` region). To provision your own customized AMI with 
[Packer](https://www.packer.io/intro/index.html), follow the instructions 
[here](aws/packer/README.md). You will need to replace the AMI ID in 
`terraform.tfvars` with your own. You can also modify the `region`, 
`instance_type`, `server_count`, and `client_count`. At least one client and
one server are required.

Provision the cluster:

```bash
$ terraform get
$ terraform plan
$ terraform apply
```

## Access the cluster

SSH to one of the servers using its public IP:

```bash
$ ssh -i /path/to/key ubuntu@PUBLIC_IP
```

Note that the AWS security group is configured by default to allow all traffic 
over port 22. This is *not* recommended for production deployments.

Run a few basic commands to verify that Consul and Nomad are up and running 
properly:

```bash
$ consul members
$ nomad server-members
$ nomad node-status
```

Optionally, initialize and unseal Vault:

```bash
$ vault init -key-shares=1 -key-threshold=1
$ vault unseal
$ export VAULT_TOKEN=[INITIAL_ROOT_TOKEN]
```

The `vault init` command above creates a single 
[Vault unseal key](https://www.vaultproject.io/docs/concepts/seal.html) for 
convenience. For a production environment, it is recommended that you create at 
least five unseal key shares and securely distribute them to independent 
operators. The `vault init` command defaults to five key shares and a key 
threshold of three. If you provisioned more than one server, the others will 
become standby nodes (but should still be unsealed). You can query the active 
and standby nodes independently:

```bash
$ dig active.vault.service.consul
$ dig active.vault.service.consul SRV
$ dig standby.vault.service.consul
``` 

## Getting started with Nomad & the HashiCorp stack

See:

* [Getting Started with Nomad](https://www.nomadproject.io/intro/getting-started/jobs.html)
* [Consul integration](https://www.nomadproject.io/docs/service-discovery/index.html)
* [Vault integration](https://www.nomadproject.io/docs/vault-integration/index.html)
* [consul-template integration](https://www.nomadproject.io/docs/job-specification/template.html) 

## Apache Spark integration

Nomad is well-suited for analytical workloads, given its performance 
characteristics and first-class support for batch scheduling. Apache Spark is a 
popular data processing engine/framework that has been architected to use 
third-party schedulers. The Nomad ecosystem includes a [fork that natively 
integrates Nomad with Spark](https://github.com/hashicorp/nomad-spark). A
detailed walkthrough of the integration is included [here](examples/spark/README.md).
