# Terraform Provisioning

These scripts are copied up to instances via Terraform provisioning and executed after launch. This allows us to update the Nomad configurations for features that land on master without having to re-bake AMIs.

## What goes here?

* steps that are specific to a given Nomad build: ex. all Nomad configuration files.
* steps that are specific to a given EC2 instance: configuring IP addresses.

These scripts *should* be idempotent: copy configurations from `/ops/shared` to their destinations where the services expect them to be, rather than moving them.
