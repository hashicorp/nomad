# Provision a Nomad cluster on GCP

[![Open in Cloud Shell](https://gstatic.com/cloudssh/images/open-btn.svg)](https://ssh.cloud.google.com/cloudshell/editor?shellonly=true&cloudshell_git_repo=https%3A%2F%2Fgithub.com%2Fhashicorp%2Fnomad&cloudshell_working_dir=terraform%2Fgcp&cloudshell_tutorial=README.md)

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

### Install and Authenticate the GCP SDK Command Line Tools

**If you are using [Google Cloud](https://cloud.google.com/shell), you already have `gcloud` setup. So, you can safely skip this step.**

To install the GCP SDK Command Line Tools, follow the installation instructions for your specific operating system: 

* [Linux](https://cloud.google.com/sdk/docs/downloads-interactive#linux)
* [MacOS](https://cloud.google.com/sdk/docs/downloads-interactive#mac)
* [Windows](https://cloud.google.com/sdk/docs/downloads-interactive#windows)

After installation, authenticate `gcloud` with the following command:

```console
gcloud auth login
```

## Create a New Project

Generate a project ID with the following command:

```console
export GOOGLE_PROJECT="nomad-gcp-$(cat /dev/random | head -c 5 | xxd -p)"
```

Using that project ID, create a new GCP [project](https://cloud.google.com/docs/overview#projects):

```console
gcloud projects create $GOOGLE_PROJECT
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

Locate the `ACCOUNT_ID` for the billing account you want to use, and set the `GOOGLE_BILLING_ACCOUNT` environment variable. Replace the `XXXXXXX` with the `ACCOUNT_ID` you located with the previous command output:

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
> The `account.json` credentials gives privileged access to this GCP project. Be careful to avoid leaking these credentials by accidentally committing them to version control systems such as `git`, or storing them where they are visible to others. In general, storing these credentials on an individually operated, private computer (like your laptop) or in your own GCP cloud shell is acceptable for testing purposes. For production use, or for teams, use a secrets management system like HashiCorp [Vault](https://www.vaultproject.io/). For this tutorial's purposes, we'll be storing the `account.json` credentials on disk in the cloud shell.

Now set the *full path* of the newly created `account.json` file as `GOOGLE_APPLICATION_CREDENTIALS` environment variable.

```console
export GOOGLE_APPLICATION_CREDENTIALS=$(realpath account.json)
```

### Ensure Required Environment Variables Are Set

Before moving onto the next steps, ensure the following environment variables are set:

* `GOOGLE_PROJECT` with your selected GCP project ID.
* `GOOGLE_APPLICATION_CREDENTIALS` with the *full path* to the Terraform Service Account `account.json` credentials file created in the last step.

## Build HashiStack Golden Image with Packer

[Packer](https://www.packer.io/intro/index.html) is HashiCorp's open source tool for creating identical machine images for multiple platforms from a single source configuration. The machine image created here can be customized through modifications to the [build configuration file](https://github.com/hashicorp/nomad/blob/master/terraform/gcp/packer.json) and the [shell script](https://github.com/hashicorp/nomad/blob/master/terraform/shared/scripts/setup.sh).

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

To access the Nomad, Consul, or Vault web UI inside the cluster, create an [SSH tunnel](https://cloud.google.com/community/tutorials/ssh-tunnel-on-gce) using `gcloud`. To open up tunnels to *all* of the UIs available in the cluster, run these commands which will start each SSH tunnel as a background process in your current shell:

```console
gcloud compute ssh hashistack-server-0 --zone=us-east1-c --tunnel-through-iap -- -f -N -L 127.0.0.1:4646:127.0.0.1:4646
gcloud compute ssh hashistack-server-0 --zone=us-east1-c --tunnel-through-iap -- -f -N -L 127.0.0.1:8200:127.0.0.1:8200
gcloud compute ssh hashistack-server-0 --zone=us-east1-c --tunnel-through-iap -- -f -N -L 127.0.0.1:8500:127.0.0.1:8500
```

After running those commands, you can now click any of the following links to open up a Web Preview using Cloud Shell:

* [Nomad](https://ssh.cloud.google.com/devshell/proxy?authuser=0&port=4646&environment_id=default)
* [Vault](https://ssh.cloud.google.com/devshell/proxy?authuser=0&port=8200&environment_id=default)
* [Consul](https://ssh.cloud.google.com/devshell/proxy?authuser=0&port=8500&environment_id=default)

If you're **not** using Cloud Shell, you can use any of these links:

* [Nomad](http://127.0.0.1:4646)
* [Vault](http://127.0.0.1:8200)
* [Consul](http://127.0.0.1:8500)


## Next Steps

Click [here](https://github.com/hashicorp/nomad/blob/master/terraform/README.md#test) for next steps.

## Conclusion

You have deployed a Nomad cluster to GCP!

### Destroy Infrastrucure

To destroy all the demo infrastrucure: 

```console
terraform destroy -force -var="project=${GOOGLE_PROJECT}" -var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 
```
