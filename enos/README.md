# Upgrade Testing with Enos

We're using [Enos](https://github.com/hashicorp/enos) to perform upgrade
testing. These tests are run via GitHub Actions from the private `nomad-e2e`
repository. This document describes how you can run these tests from your local
development environment if you're a HashiCorp developer.

There are two major components to be aware of:
* This directory includes the upgrade scenario and the Terraform modules and
  shell scripts needed to execute that scenario.
* The scenario uses the same cluster provisioning infrastructure as the E2E
  tests in the `e2e/` directory in the root of this repo. So to run the upgrade
  scenario you also have to have all the credentials set up to run the E2E
  tests. (We may try to fold these together in the future.)

The `terraform/` folder has provisioning code to spin up a Nomad cluster on
AWS. You'll need both Terraform and AWS credentials to setup AWS instances on
which e2e tests will run. See the
[README](https://github.com/hashicorp/nomad/blob/main/e2e/terraform/README.md)
for details. The number of servers and clients is configurable, as is the
specific build of Nomad to deploy and the configuration file for each client
and server.

## Setup

You'll need a recent version of Terraform, the most current version of Enos, as
well as AWS credentials to create the Nomad cluster and credentials for HCP. The
Terraform configurations assume that an appropriate instance role has been
configured elsewhere and that you have the ability to `AssumeRole` into the AWS
account.

Configure the following environment variables. For HashiCorp Nomad developers,
this configuration can be found in 1Pass in the Nomad team's vault under
`nomad-e2e`.

```
export HCP_CLIENT_ID=
export HCP_CLIENT_SECRET=
```

The Vault admin token will expire after 6 hours. If you haven't created one
already use the separate Terraform configuration found in the
`$REPO/e2e/terraform/hcp-vault-auth` directory. The following will set the correct
values for `VAULT_TOKEN`, `VAULT_ADDR`, and `VAULT_NAMESPACE`:

```
terraform init
terraform apply --auto-approve
$(terraform output --raw environment)
```

Make sure your AWS credentials have been refreshed with the appropriate IAM role:

```
$ doormat login --force
$ doormat aws cred-file add-profile --role "$ROLE" --set-default
```

Next you'll need to obtain an Artifactory token via Doormat.

```
export ARTIFACTORY_TOKEN=$(doormat artifactory create-token | jq -r .access_token)
```

Next you'll need to populate the Enos variables file `enos.vars.hcl (unlike
Terraform, Enos doesn't accept variables on the command line):

```hcl
artifactory_username = "<your email address>"
artifactory_token    = "<your ARTIFACTORY_TOKEN from above>"
product_version      = "1.8.9"                        # starting version
upgrade_version      = "1.9.4"                        # version to upgrade to
download_binary_path = "/home/foo/Downloads/nomad"    # directory on your machine to download binaries
nomad_license        = "<your Nomad Enterprise license, when running Nomad ENT>"
consul_license       = "<your Consul Enterprise license, currently always required>"
aws_region           = "us-east-1"
```

When the variables file is placed in the enos root folder with the name 
`enos.vars.hcl` it is automatically picked up by enos, if a different variables 
files will be used, it can be pass using the flag `--var-file`.

## Reviewing Enos

You can quickly validate the Enos scenario configuration without running it:

```
$ enos scenario validate upgrade --var-file /tmp/enos.vars
$ echo $?
0
```

You can also review what Enos will do by generating an outline you can read in
your browser:

```
$ enos scenario outline upgrade --var-file /tmp/enos.vars --format=html > /tmp/outline.html
$ open /tmp/outline.html
```

## Running Enos

Run the Enos scenario end-to-end:

```
$ enos scenario run upgrade --var-file /tmp/enos.vars --timeout 2h
```

Enos will not clean up after itself automatically if interrupted. If you have to
interrupt it, you may need to run `enos scenario destroy upgrade --var-file
/tmp/enos.vars `
