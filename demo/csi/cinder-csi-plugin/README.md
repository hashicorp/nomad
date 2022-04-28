# Openstack Cinder CSI-Plugin

## Requirements

The containers that run the Node/Controller applications require a cloud-config file be mounted in the containers and the path specified in the containers `args`.
  
The example plugin job creates a file at `local/cloud.conf` using a [`template`](https://www.nomadproject.io/docs/job-specification/template) stanza which pulls the necessary credentials from a [Vault kv-v2](https://www.vaultproject.io/docs/secrets/kv/kv-v2) secrets store. However, other methods, such as using the [`artifact`](https://www.nomadproject.io/docs/job-specification/artifact) stanza, will work as well for delivering the `cloud.conf` file to the CSI drivers.

### Example cloud.conf

```ini
[Global]
username = openstack-user
password =  superSecret123
domain-name = default
auth-url = https://service01a-c2.example.com:5001/
tenant-id = 5sd6f4s5df6sd6fs5ds65fd4f65s
region = RegionOne
```

### Docker Privileged Mode

The Cinder CSI Node task requires that [`privileged = true`](https://www.nomadproject.io/docs/drivers/docker#privileged) be set. This is not needed for the Controller task.

## Container Arguments

- `--endpoint=unix:///csi/csi.sock`

  - This option must match the `mount_dir` specified in the `csi_plugin` stanza for the task.

- `--cloud-config=/etc/config/cloud.conf`
  
  - The location that the cloud.conf file was mounted inside the container
  
- `--nodeid=${node.unique.name}`

  - A unique ID for the node the task is running on. Recommend using `${node.unique.name}`

- `--cluster=${NOMAD_DC}`

  - The cluster the Controller/Node is a part of. Recommend using `${NOMAD_DC}`

## Deployment

### Plugin

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad job run cinder-csi-plugin.hcl
```

### Volume Registration

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad volume register example_volume.hcl
```

## Cinder CSI Driver Source

- https://github.com/kubernetes/cloud-provider-openstack/tree/master/pkg/csi/cinder
