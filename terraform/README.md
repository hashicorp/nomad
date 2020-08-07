# Provision a Nomad cluster in the Cloud

Use this repo to easily provision a Nomad sandbox environment on AWS, Azure, or GCP with 
[Packer](https://packer.io) and [Terraform](https://terraform.io). 
[Consul](https://www.consul.io/intro/index.html) and 
[Vault](https://www.vaultproject.io/intro/index.html) are also installed 
(colocated for convenience). The intention is to allow easy exploration of 
Nomad and its integrations with the HashiCorp stack. This is *not* meant to be
a production ready environment. A demonstration of [Nomad's Apache Spark 
integration](examples/spark/README.md) is included. 

## Setup

Clone the repo and optionally use [Vagrant](https://www.vagrantup.com/intro/index.html) 
to bootstrap a local staging environment:

```bash
$ git clone git@github.com:hashicorp/nomad.git
$ cd nomad/terraform
$ vagrant up && vagrant ssh
```

The Vagrant staging environment pre-installs Packer, Terraform, Docker and the 
Azure CLI.

## Provision a cluster

- Follow the steps [here](aws/README.md) to provision a cluster on AWS.
- Follow the steps [here](azure/README.md) to provision a cluster on Azure.
- Follow the steps [here](gcp/README.md) to provision a cluster on GCP.

Continue with the steps below after a cluster has been provisioned.

## Test

Run a few basic status commands to verify that Consul and Nomad are up and running 
properly:

```bash
$ consul members
$ nomad server members
$ nomad node status
```

## Unseal the Vault cluster (optional)

To initialize and unseal Vault, run:

```bash
$ vault operator init -key-shares=1 -key-threshold=1
$ vault operator unseal
$ export VAULT_TOKEN=[INITIAL_ROOT_TOKEN]
```

The `vault init` command above creates a single 
[Vault unseal key](https://www.vaultproject.io/docs/concepts/seal.html) for 
convenience. For a production environment, it is recommended that you create at 
least five unseal key shares and securely distribute them to independent 
operators. The `vault init` command defaults to five key shares and a key 
threshold of three. If you provisioned more than one server, the others will 
become standby nodes but should still be unsealed. You can query the active 
and standby nodes independently:

```bash
$ dig active.vault.service.consul
$ dig active.vault.service.consul SRV
$ dig standby.vault.service.consul
```

See the [Getting Started guide](https://www.vaultproject.io/intro/getting-started/first-secret.html) 
for an introduction to Vault.

## Getting started with Nomad & the HashiCorp stack

Use the following links to get started with Nomad and its HashiCorp integrations:

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
