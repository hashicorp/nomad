# AWS EBS CSI Plugin

The configuration here is for the [AWS Elastic Block Store](https://aws.amazon.com/ebs/) CSI driver.

## Requirements

The example plugin jobs use [templates](https://www.nomadproject.io/docs/job-specification/template) rendered with secrets obtained from the [Vault AWS Secrets Engine](https://www.vaultproject.io/docs/secrets/aws) to obtain the credentials necessary to control EBS volumes.
Additionally this directory contains some sample Vault and AWS IAM (via [Terraform](https://www.terraform.io)) configuration that may be useful as a starting point for setting up Vault to provide credentials for these jobs.

### Docker Privileged Mode

The EBS node task requires that [`privileged = true`](https://www.nomadproject.io/docs/drivers/docker#privileged) be set in order to mount/unmount filesystems on the node.

### EC2 Instance Metadata Service.

The driver also requires access to the [EC2 Instance Metadata Service](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html).
If you have blocked access to this from docker tasks running in bridge (default) or NAT mode, which is a good idea for security, you may run the task in host networking mode by setting [`network_mode = host`](https://www.nomadproject.io/docs/drivers/docker#network_mode)

## Container Arguments
 
Refer to the official plugin [README](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/README.md).

- `controller`
 
  - Run the driver in controller mode.

- `node`

  - Run the driver in node mode.

- `--endpoint=unix:///csi/csi.sock`

  - This option must match the `mount_dir` specified in the `csi_plugin` stanza for the task.

- --logtostderr

  - Logs are written to standard error instead of to files.

- --v=5

  - Sets the klog logging "V" level to 5.

## Deployment

### Plugins

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad job run plugin-aws-ebs-controller.hcl
nomad job run plugin-aws-ebs-nodes.hcl
```

### Volume Registration

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad volume register example-volume.hcl
```

## EBS CSI Driver Source

- https://github.com/kubernetes-sigs/aws-ebs-csi-driver
