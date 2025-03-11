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

```sh
terraform init
terraform apply --auto-approve
$(terraform output --raw environment)
```

Make sure your AWS credentials have been refreshed with the appropriate IAM role:

```sh
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

```sh
$ enos scenario validate upgrade --var-file /tmp/enos.vars
$ echo $?
0
```

You can also review what Enos will do by generating an outline you can read in
your browser:

```sh
$ enos scenario outline upgrade --var-file /tmp/enos.vars --format=html > /tmp/outline.html
$ open /tmp/outline.html
```

## Running Enos

Run the Enos scenario end-to-end:

```sh
$ enos scenario run upgrade --var-file /tmp/enos.vars --timeout 2h
```

Enos will not clean up after itself automatically if interrupted. If you have to
interrupt it, you may need to run `enos scenario destroy upgrade --var-file
/tmp/enos.vars `

## Debugging

Enos builds Terraform state in the `.enos` directory, in a subdirectory named
with a hash. If you're working on Enos scenarios or test workloads and want to
connect to the Nomad cluster you create, you can use the `debug-environment`
script in this directory to set your Nomad environment variables by passing it
the path to that subdirectory. For example:

```sh
$ $(./debug-environment .enos/c545bbc25c5eec0ca86c99595a9034b5451a91aa10b586da2baab435df65be2e)
```

Note that this won't be fully populated until the Enos scenario is far enough
along to bootstrap the Nomad cluster.

## Adding New Workloads

All workloads executed as part of the test suite are stored under 
`enos/modules/run_workloads/jobs` and must be defined with the following 
attributes:

### Required Attributes

- **`job_spec`** *(string)*: Path to the job specification for your workload.
 The path should be relative to the `run_workloads` module. 
 For example: `jobs/raw-exec-service.nomad.hcl`.

- **`alloc_count`** *(number)*: This variable serves two purposes:
  1. Every workload must define the `alloc_count` variable, regardless of 
  whether it is actively used.
   This is because jobs are executed using [this command](https://github.com/hashicorp/nomad/blob/1ffb7ab3fb0dffb0e530fd3a8a411c7ad8c72a6a/enos/modules/run_workloads/main.tf#L66):
     
     ```hcl
     variable "alloc_count" {
       type = number
     }
     ```
  This is done to force the job spec author to add a value to the `alloc_count`.
  2. It is used to calculate the expected number of allocations in the cluster 
  once all jobs are running.
     
     If the variable is missing or left undefined, the job will fail to run, 
     which will impact the upgrade scenario.
     
     For `system` jobs, the number of allocations is determined by the number 
     of nodes. In such cases, `alloc_count` is conventionally set to `0`,
    as it is not directly used.

- **`type`** *(string)*: Specifies the type of workload—`service`, `batch`, or 
`system`. Setting the correct type is important, as it affects the calculation
of the total number of expected allocations in the cluster.

### Optional Attributes

The following attributes are only required if your workload has prerequisites 
or final configurations before it is fully operational. For example, a job using
`tproxy` may require a new intention to be configured in Consul.

- **`pre_script`** *(optional, string)*: Path to a script that should be 
executed before the job runs.
- **`post_script`** *(optional, string)*: Path to a script that should be
 executed after the job runs.
  
All scripts are located in `enos/modules/run_workloads/scripts`.
Similar to `job_spec`, the path should be relative to the `run_workloads`
module. Example: `scripts/wait_for_nfs_volume.sh`.

### Adding a New Workload

If you want to add a new workload to test a specific feature, follow these steps:

1. Modify the `run_initial_workloads` [step](https://github.com/hashicorp/nomad/blob/main/enos/enos-scenario-upgrade.hcl) 
in `enos-scenario-upgrade.hcl` and include your workload in the `workloads` 
variable.

2. Add the job specification and any necessary pre/post scripts to the
appropriate directories:
   - [`enos/modules/run_workloads/jobs`](https://github.com/hashicorp/nomad/tree/main/enos/modules/run_workloads/jobs)
   - [`enos/modules/run_workloads/scripts`](https://github.com/hashicorp/nomad/tree/main/enos/modules/run_workloads/scripts)

**Important:** Ensure that the `alloc_count` variable is included in the job
specification. If it is missing or undefined, the job will fail to run, 
potentially disrupting the upgrade scenario.

If you want to verify your workload without having to run all the scenario, 
you can manually pass values to variables with flags or a `.tfvars`
file and run the module from the `run_workloads` directory like you would any
other terraform module.

