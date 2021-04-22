# CSI on Hetzner Cloud

This is a Terraform demo for deploying CSI volumes on Hetzner Cloud. It
assumes you already have a Nomad cluster running with the Docker task
driver. You will need a Hetzner Cloud account and a Hetzner Cloud API key.

Deploy the demo:

```
export NOMAD_ADDR=http://${IP_ADDRESS}:4646
terraform apply -var hcloud_token=${TOKEN}
```
