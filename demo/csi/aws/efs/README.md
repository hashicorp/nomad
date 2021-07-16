# AWS EFS CSI Plugin

The configuration here is for the [AWS Elastic Filesystem](https://aws.amazon.com/efs/) CSI driver. This driver only needs to run on nodes, there is no controller required.

## Requirements

### Docker Privileged Mode

The EFS node task requires that [`privileged = true`](https://www.nomadproject.io/docs/drivers/docker#privileged) be set in order to mount/unmount filesystems on the node.

### EC2 Instance Metadata Service.

The driver also requires access to the [EC2 Instance Metadata Service](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html).
If you have blocked access to this from docker tasks running in bridge (default) or NAT mode, which is a good idea for security, you may run the task in host networking mode by setting [`network_mode = host`](https://www.nomadproject.io/docs/drivers/docker#network_mode)

## Container Arguments
 
Refer to the official plugin [README](https://github.com/kubernetes-sigs/aws-efs-csi-driver/blob/master/docs/README.md).
 
- `--endpoint=unix:///csi/csi.sock`

  - This option must match the `mount_dir` specified in the `csi_plugin` stanza for the task.

- `--logtostderr`

  - Logs are written to standard error instead of to files.

- `--v=5`

  - Sets the klog logging "V" level to 5.

## Deployment

### Plugin

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad job run plugin-aws-efs-nodes.hcl
```

### Volume Registration

Note that it is possible to use the root of the filesystem or a subdirectory, and it is perfectly fine to have different volumes using overlapping subtrees (eg `/`, `/foo`, and `/foo/bar`), or even to have multiple volumes use the same path.

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad volume register example-volume.hcl
```

## EFS CSI Driver Source

- https://github.com/kubernetes-sigs/aws-efs-csi-driver
