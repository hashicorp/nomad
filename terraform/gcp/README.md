# Provision a Nomad cluster on GCP

<!-- TODO: change to use default branch before merging -->
[![Open in Cloud Shell](https://gstatic.com/cloudssh/images/open-btn.svg)](https://ssh.cloud.google.com/cloudshell/editor?shellonly=true&cloudshell_git_repo=https%3A%2F%2Fgithub.com%2Fhashicorp%2Fnomad&cloudshell_git_branch=getting-started-on-gcp&cloudshell_working_dir=terraform%2Fgcp&cloudshell_tutorial=README.md)

To get started, you will need a GCP [account](https://cloud.google.com/free).

## Welcome

This tutorial will teach you how to deploy [Nomad](https://www.nomadproject.io/) clusters to the Google Cloud Platform using [Packer](https://www.packer.io/) and [Terraform](https://www.terraform.io/).

Includes:

* Installing HashiCorp Tools (Nomad, Consul, Vault, Terraform, and Packer).
* Installing the GCP SDK CLI Tools, if you're not using Cloud Shell.
* Creating a new GCP project, along with a Terraform Service Account.
* Building a golden image using Packer.
* Deploying a cluster with Terraform.

## Install HashiCorp Tools

### Nomad

Download the latest version of [Nomad](https://www.nomadproject.io/) from HashiCorp's website by copying and pasting this snippet in the terminal:

```console
curl "https://releases.hashicorp.com/nomad/0.12.0/nomad_0.12.0_linux_amd64.zip" -o nomad.zip
unzip nomad.zip
sudo mv nomad /usr/local/bin
nomad --version
```

### Consul

Download the latest version of [Consul](https://www.consul.io/) from HashiCorp's website by copying and pasting this snippet in the terminal:

```console
curl "https://releases.hashicorp.com/consul/1.8.0/consul_1.8.0_linux_amd64.zip" -o consul.zip
unzip consul.zip
sudo mv consul /usr/local/bin
consul --version
```

### Vault

Download the latest version of [Vault](https://www.vaultproject.io/) from HashiCorp's website by copying and pasting this snippet in the terminal:

```console
curl "https://releases.hashicorp.com/vault/1.4.3/vault_1.4.3_linux_amd64.zip" -o vault.zip
unzip vault.zip
sudo mv vault /usr/local/bin
vault --version
```

### Packer

Download the latest version of [Packer](https://www.packer.io/) from HashiCorp's website by copying and pasting this snippet in the terminal:

```console
curl "https://releases.hashicorp.com/packer/1.6.0/packer_1.6.0_linux_amd64.zip" -o packer.zip
unzip packer.zip
sudo mv packer /usr/local/bin
packer --version
```

### Terraform

Download the latest version of [Terraform](https://www.terraform.io/) from HashiCorp's website by copying and pasting this snippet in the terminal:

```console
curl "https://releases.hashicorp.com/terraform/0.12.28/terraform_0.12.28_linux_amd64.zip" -o terraform.zip
unzip terraform.zip
sudo mv terraform /usr/local/bin
terraform --version
```

### Install the GCP SDK Command Line Tools

> **Note**: if you are using the free [Google Cloud Shell](https://cloud.google.com/shell) VM, you will already have `gcloud` installed. So, you can safley skip this step.

To install the GCP SDK Command Line Tools, follow the installation instructions for your specific operating system: 

* [Linux](https://cloud.google.com/sdk/docs/downloads-interactive#linux)
* [MacOS](https://cloud.google.com/sdk/docs/downloads-interactive#mac)
* [Windows](https://cloud.google.com/sdk/docs/downloads-interactive#windows)

#### Initialize the SDK

To perform common setup tasks like authorizing with GCP using `gcloud`, if you haven't already done so, run the following command:

```console
gcloud init
```

## Create a New Project

Create a new GCP project with the following command:

```console
gcloud projects create your-new-project-name
```

Now export the project name as the `GOOGLE_PROJECT` environment variable:

```console
export GOOGLE_PROJECT="your-new-project-name"
```

And then set your `gcloud` config to use that project:

```console
gcloud config set project $GOOGLE_PROJECT
```

### Link Billing Account to Project

Next, let's link a billing account to that project. To determine what billing accounts are available, run the following command:

```console
gcloud alpha billing accounts list
```

Then set the billing account ID `GOOGLE_BILLING_ACCOUNT` environment variable:

```console
export GOOGLE_BILLING_ACCOUNT="XXXXXXX"
```

So we can link the `GOOGLE_BILLING_ACCOUNT` with the previously created `GOOGLE_PROJECT`:

```console
gcloud alpha billing projects link "$GOOGLE_PROJECT" --billing-account "$GOOGLE_BILLING_ACCOUNT"
```

### Enable Compute API

In order to deploy VMs to the project, we need to enable the compute API:

```console
gcloud services enable compute.googleapis.com
```

### Create Terraform Service Account

Finally, let's create a Terraform Service Account user and its `account.json` credentials file:

```console
gcloud iam service-accounts create terraform \
    --display-name "Terraform Service Account" \
    --description "Service account to use with Terraform"
```

```console
gcloud projects add-iam-policy-binding "$GOOGLE_PROJECT" \
  --member serviceAccount:"terraform@$GOOGLE_PROJECT.iam.gserviceaccount.com" \
  --role roles/editor
```

```console
gcloud iam service-accounts keys create account.json \
    --iam-account "terraform@$GOOGLE_PROJECT.iam.gserviceaccount.com"
```

> ⚠️ **Warning**
>
> The `account.json` credentials gives privelleged access to this GCP project. Be sure to prevent from accidently leaking these credentials in version control systems such as `git`. In general, as an operator on your own host machine, or in your own GCP cloud shell is ok. However, using a secrets management system like HashiCorp [Vault](https://www.vaultproject.io/) can often be a better solution for teams. For this tutorial's purposes, we'll be storing the `account.json` credentials on disk in the cloud shell.

Now set the *full path* of the newly created `account.json` file as `GOOGLE_APPLICATION_CREDENTIALS` environment variable.

```console
export GOOGLE_APPLICATION_CREDENTIALS=$(realpath account.json)
```

### Ensure Required Environment Variables Are Set

Before moving onto the next steps, ensure the following environment variables are set:

* `GOOGLE_PROJECT` with your selected GCP project name.
* `GOOGLE_APPLICATION_CREDENTIALS` with the *full path* to the Terraform Service Account `account.json` credentials file created with the last step.

## Build HashiStack Golden Image with Packer

[Packer](https://www.packer.io/intro/index.html) is HashiCorp's open source tool  for creating identical machine images for multiple platforms from a single  source configuration. The machine image created here can be customized through modifications to the [build configuration file](packer.json) and the [shell script](../shared/scripts/setup.sh).

Use the following command to build the machine image:

```console
packer build packer.json
```

## Provision a cluster with Terraform

Change into the `env/us-east` environment directory:

```console
cd env/us-east
```

Initialize Terraform:

```console
terraform init
```

Plan infrastructure changes with Terraform:

```console
terraform plan -var="project=${GOOGLE_PROJECT}" -var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 
```

Apply infrastructure changes with Terraform:

```console
terraform apply -auto-approve -var="project=${GOOGLE_PROJECT}" -var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 
```

## Access the Cluster

You can now access the cluster in serveral ways.

### UI

Put the `hashistack_load_balancer_external_ip` Terraform Output in your web browser to access the UI.

### CLI

Export the `hashistack_load_balancer_external_ip` Terraform Output as an environment variable:

```console
export HASHISTACK_LB_EXTERNAL_IP=$(terraform output -json | jq -r '.hashistack_load_balancer_external_ip.value')
export NOMAD_ADDR="http://$HASHISTACK_LB_EXTERNAL_IP:4646"
export CONSUL_HTTP_ADDR="http://$HASHISTACK_LB_EXTERNAL_IP:8500"
export VAULT_ADDR="http://$HASHISTACK_LB_EXTERNAL_IP:8200"
```

```console
nomad server members
```

```console
consul members
```

### SSH

Use `gcloud` to SSH into one of the servers:

```bash
gcloud compute ssh hashistack-server-0 --zone=us-east1-c
```

## Next Steps

Click [here](../README.md#test) for next steps.