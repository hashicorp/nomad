# CSI on DigitalOcean

This is a Terraform demo for deploying CSI volumes on DigitalOcean. It
asssumes you already have a Nomad cluster running with the Docker task
driver. You will need a DigitalOcean account and a DigitalOcean API key.

Deploy the demo:

```
export NOMAD_ADDR=http://${IP_ADDRESS}:4646
terraform apply -var do_token=${DIGITALOCEAN_TOKEN}
```

See the volume is registered:

```
$ nomad volume status nomad-csi
ID                   = nomad-csi-test
Name                 = nomad-csi-test
External ID          = 58c4ef75-25d1-11eb-a381-0a58ac1449b9
Plugin ID            = digitalocean
Provider             = dobs.csi.digitalocean.com
Version              = v2.1.1
Schedulable          = true
Controllers Healthy  = 1
Controllers Expected = 1
Nodes Healthy        = 1
Nodes Expected       = 1
Access Mode          = single-node-writer
Attachment Mode      = block-device
Mount Options        = <none>
Namespace            = default

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created  Modified
8d223dc7  ce46add9  cache       0        run      running  21s ago  3s ago
```
