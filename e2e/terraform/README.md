# Terraform infrastructure

This folder contains Terraform resources for provisioning a Nomad cluster on
EC2 instances on AWS to use as the target of end-to-end tests.

Terraform provisions the AWS infrastructure assuming that EC2 AMIs have
already been built via Packer. It deploys a specific build of Nomad to the
cluster along with configuration files for Nomad, Consul, and Vault.

## Setup

You'll need Terraform 0.14.7+, as well as AWS credentials to create the Nomad
cluster. This Terraform stack assumes that an appropriate instance role has
been configured elsewhere and that you have the ability to `AssumeRole` into
the AWS account.

Optionally, edit the `terraform.tfvars` file to change the number of Linux
clients or Windows clients. The Terraform variables file
`terraform.full.tfvars` is for the nightly E2E test run and deploys a larger,
more diverse set of test targets.

```hcl
region                           = "us-east-1"
instance_type                    = "t2.medium"
server_count                     = "3"
client_count_ubuntu_bionic_amd64 = "4"
client_count_windows_2016_amd64  = "1"
profile                          = "dev-cluster"
```

Run Terraform apply to deploy the infrastructure:

```sh
cd e2e/terraform/
terraform apply
```

> Note: You will likely see "Connection refused" or "Permission denied" errors
> in the logs as the provisioning script run by Terraform hits an instance
> where the ssh service isn't yet ready. That's ok and expected; they'll get
> retried. In particular, Windows instances can take a few minutes before ssh
> is ready.

## Nomad Version

You'll need to pass one of the following variables in either your
`terraform.tfvars` file or as a command line argument (ex. `terraform apply
-var 'nomad_version=0.10.2+ent'`)

* `nomad_local_binary`: provision this specific local binary of Nomad. This is
  a path to a Nomad binary on your own host. Ex. `nomad_local_binary =
  "/home/me/nomad"`. This setting overrides `nomad_version`.
* `nomad_url`: provision this version from a remote archived binary, e.g. `build-binaries` CircleCI artifacts zip file urls.
* `nomad_version`: provision this version from
  [releases.hashicorp.com](https://releases.hashicorp.com/nomad). Ex. `nomad_version
  = "0.10.2+ent"`

If you want to deploy the Enterprise build, include `-var
'nomad_enterprise=true'`.

If you want to bootstrap Nomad ACLs, include `-var 'nomad_acls=true'`.

> Note: If you bootstrap ACLs you will see "No cluster leader" in the output
> several times while the ACL bootstrap script polls the cluster to start and
> and elect a leader.

## Profiles

The `profile` field selects from a set of configuration files for Nomad,
Consul, and Vault by uploading the files found in `./config/<profile>`. The
standard profiles are as follows:

* `full-cluster`: This profile is used for nightly E2E testing. It assumes at
  least 3 servers and includes a unique config for each Nomad client.
* `dev-cluster`: This profile is used for developer testing of a more limited
  set of clients. It assumes at least 3 servers but uses the one config for
  all the Linux Nomad clients and one config for all the Windows Nomad
  clients.

You may create additional profiles for testing more complex interactions between features.
You can build your own custom profile by writing config files to the
`./config/<custom name>` directory.

For each profile, application (Nomad, Consul, Vault), and agent type
(`server`, `client_linux`, or `client_windows`), the agent gets the following
configuration files, ignoring any that are missing.

* `./config/<profile>/<application>/*`: base configurations shared between all
  servers and clients.
* `./config/<profile>/<application>/<type>/*`: base configurations shared
  between all agents of this type.
* `./config/<profile>/<application>/<type>/indexed/*<index>.<ext>`: a
  configuration for that particular agent, where the index value is the index
  of that agent within the total count.

For example, with the `full-cluster` profile, 2nd Nomad server would get the
following configuration files:
* `./config/full-cluster/nomad/base.hcl`
* `./config/full-cluster/nomad/server/indexed/server-1.hcl`

The directory `./config/full-cluster/nomad/server` has no configuration files,
so that's safely skipped.

## Outputs

After deploying the infrastructure, you can get connection information
about the cluster:

- `$(terraform output --raw environment)` will set your current shell's
  `NOMAD_ADDR` and `CONSUL_HTTP_ADDR` to point to one of the cluster's server
  nodes, and set the `NOMAD_E2E` variable.
- `terraform output servers` will output the list of server node IPs.
- `terraform output linux_clients` will output the list of Linux
  client node IPs.
- `terraform output windows_clients` will output the list of Windows
  client node IPs.

## SSH

You can use Terraform outputs above to access nodes via ssh:

```sh
ssh -i keys/nomad-e2e-*.pem ubuntu@${EC2_IP_ADDR}
```

The Windows client runs OpenSSH for convenience, but has a different
user and will drop you into a Powershell shell instead of bash:

```sh
ssh -i keys/nomad-e2e-*.pem Administrator@${EC2_IP_ADDR}
```

## Teardown

The terraform state file stores all the info.

```sh
cd e2e/terraform/
terraform destroy
```

## FAQ

#### E2E Provisioning Goals

1. The provisioning process should be able to run a nightly build against a
  variety of OS targets.
2. The provisioning process should be able to support update-in-place
  tests. (See [#7063](https://github.com/hashicorp/nomad/issues/7063))
3. A developer should be able to quickly stand up a small E2E cluster and
  provision it with a version of Nomad they've built on their laptop. The
  developer should be able to send updated builds to that cluster with a short
  iteration time, rather than having to rebuild the cluster.

#### Why not just drop all the provisioning into the AMI?

While that's the "correct" production approach for cloud infrastructure, it
creates a few pain points for testing:

* Creating a Linux AMI takes >10min, and creating a Windows AMI can take
  15-20min. This interferes with goal (3) above.
* We won't be able to do in-place upgrade testing without having an in-place
  provisioning process anyways. This interferes with goals (2) above.

#### Why not just drop all the provisioning into the user data?

* Userdata is executed on boot, which prevents using them for in-place upgrade
  testing.
* Userdata scripts are not very observable and it's painful to determine
  whether they've failed or simply haven't finished yet before trying to run
  tests.
