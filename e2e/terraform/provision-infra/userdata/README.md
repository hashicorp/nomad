# userdata

These scripts are copied up to instances via Terraform provisioning and
executed once on first boot by
[`cloud-init`](https://cloudinit.readthedocs.io/en/latest/). Userdata scripts
should contain configuration specific to an instance but not configuration
specific to a given deployment of Nomad.
