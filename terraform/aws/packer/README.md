# Build an Amazon machine image

See the pre-requisites listed [here](../aws/README.md). If not, you will need to [install Packer](https://www.packer.io/intro/getting-started/install.html).

Set environment variables for your AWS credentials:

```bash
$ export AWS_ACCESS_KEY_ID=[ACCESS_KEY_ID]
$ export AWS_SECRET_ACCESS_KEY=[SECRET_ACCESS_KEY]
```

Build your AMI:

```bash
packer build packer.json
```

Don't forget to copy the AMI Id to your [terraform.tfvars file](../env/us-east/terraform.tfvars).
