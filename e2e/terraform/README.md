# Terraform infrastructure

This folder contains Terraform resources for provisioning a Nomad
cluster on EC2 instances on AWS to use as the target of end-to-end
tests.

Terraform provisions the AWS infrastructure assuming that EC2 AMIs
have already been built via Packer and HCP Consul and HCP Vault
clusters are already running. It deploys a build of Nomad from your
local machine along with configuration files.

## Setup

You'll need a recent version of Terraform (1.1+ recommended), as well
as AWS credentials to create the Nomad cluster and credentials for
HCP. This Terraform stack assumes that an appropriate instance role
has been configured elsewhere and that you have the ability to
`AssumeRole` into the AWS account.

Configure the following environment variables. For HashiCorp Nomad
developers, this configuration can be found in 1Pass in the Nomad
team's vault under `nomad-e2e`.

```
export HCP_CLIENT_ID=
export HCP_CLIENT_SECRET=
export CONSUL_HTTP_TOKEN=
export CONSUL_HTTP_ADDR=
```

The Vault admin token will expire after 6 hours. If you haven't
created one already use the separate Terraform configuration found in
the `hcp-vault-auth` directory. The following will set the correct
values for `VAULT_TOKEN`, `VAULT_ADDR`, and `VAULT_NAMESPACE`:

```
cd ./hcp-vault-auth
terraform init
terraform apply --auto-approve
$(terraform output --raw environment)
```

Optionally, edit the `terraform.tfvars` file to change the number of
Linux clients or Windows clients.

```hcl
region                           = "us-east-1"
instance_type                    = "t2.medium"
server_count                     = "3"
client_count_ubuntu_jammy_amd64  = "4"
client_count_windows_2016_amd64  = "1"
```

Optionally, edit the `nomad_local_binary` variable in the
`terraform.tfvars` file to change the path to the local binary of
Nomad you'd like to upload.

Run Terraform apply to deploy the infrastructure:

```sh
cd e2e/terraform/
terraform init
terraform apply
```

> Note: You will likely see "Connection refused" or "Permission denied" errors
> in the logs as the provisioning script run by Terraform hits an instance
> where the ssh service isn't yet ready. That's ok and expected; they'll get
> retried. In particular, Windows instances can take a few minutes before ssh
> is ready.
>
> Also note: When ACLs are being bootstrapped, you may see "No cluster
> leader" in the output several times while the ACL bootstrap script
> polls the cluster to start and and elect a leader.

## Configuration

The files in `etc` are template configuration files for Nomad and the
Consul agent. Terraform will render these files to the `uploads`
folder and upload them to the cluster during provisioning.

* `etc/nomad.d` are the Nomad configuration files.
  * `base.hcl`, `tls.hcl`, `consul.hcl`, and `vault.hcl` are shared.
  * `server-linux.hcl`, `client-linux.hcl`, and `client-windows.hcl` are role and platform specific.
  * `client-linux-0.hcl`, etc. are specific to individual instances.
* `etc/consul.d` are the Consul agent configuration files.
* `etc/acls` are ACL policy files for Consul and Vault.

## Web UI

To access the web UI, deploy a reverse proxy to the cluster. All
clients have a TLS proxy certificate at `/etc/nomad.d/tls_proxy.crt`
and a self-signed cert at `/etc/nomad.d/self_signed.crt`. See
`../ui/inputs/proxy.nomad` for an example of using this. Deploy as follows:

```sh
nomad namespace apply proxy
nomad job run ../ui/input/proxy.nomad
```

You can get the public IP for the proxy allocation from the following
nested query:

```sh
nomad node status -json -verbose \
    $(nomad operator api '/v1/allocations?namespace=proxy' | jq -r '.[] | select(.JobID == "nomad-proxy") | .NodeID') \
    | jq '.Attributes."unique.platform.aws.public-ipv4"'
```

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

## SSM

You can also access nodes -- including those built in CI,
where you don't have the SSH keys -- via AWS Session Manager,
with these prereqs:

- aws cli v2
  - [aws docs](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
  - [brew](https://formulae.brew.sh/formula/awscli)
- [session manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
- aws credentials in your environment

The commands are provided in Terraform Apply output, e.g.:

```sh
export AWS_REGION=us-east-1
aws ssm start-session --target i-asdf1234
```

Servers also include Nomad env vars in `/root/.bashrc`:

```
local ~ $ aws ssm start-session --target i-asdf1234

Starting session with SessionId: [.....]

$ sudo -i
root@ip-172-31-90-42:~# nomad status
No running jobs
root@ip-172-31-90-42:~# journalctl -u nomad
[... logs logs logs ...]
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
