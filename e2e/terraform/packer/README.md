# Packer Builds

These builds are run as-needed to update the AMIs used by the end-to-end test infrastructure.


## What goes here?

* steps that aren't specific to a given Nomad build: ex. all Linux instances need `jq` and `awscli`.
* steps that aren't specific to a given EC2 instance: nothing that includes an IP address.
* steps that infrequently change: the version of Consul or Vault we ship.


## Running Packer builds

```sh
$ packer --version
1.4.4

# build linux AMI
$ packer build packer.json
```
