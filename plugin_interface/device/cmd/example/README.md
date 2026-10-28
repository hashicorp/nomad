This package provides an example implementation of a device plugin for
reference.

# Behavior

The example device plugin models files within a specified directory as devices. The plugin will periodically scan the directory for changes and will expose them via the streaming Fingerprint RPC. Device health is set to unhealthy if the file has a specific filemode permission as described by the config `unhealthy_perm`. Further statistics are also collected on the detected devices.

# Installation

```shell
nomad_plugin_dir='/opt/nomad/plugins' # for example
go build -o $nomad_plugin_dir/nomad-device-example ./cmd
```

# Config

Example client agent config with our
[plugin](https://developer.hashicorp.com/nomad/docs/configuration/plugin) block:

```hcl
client {
  enabled = true
}

plugin_dir = "/opt/nomad/plugins"

plugin "nomad-device-example" {
  config {
    dir            = "/tmp/nomad-device"
    list_period    = "1s"
    unhealthy_perm = "-rwxrwxrwx"
  }
}
```

The valid configuration options are:

* `dir` (`string`: `"."`): The directory to scan for files that will represent fake devices.
* `list_period` (`string`: `"5s"`): The interval to scan the directory for changes.
* `unhealthy_perm` (`string`: `"-rwxrwxrwx"`): The file mode permission that if set on a detected file will casue the device to be considered unhealthy.

# Usage

Create two instances of the device, one unhealthy:

```shell
mkdir -p /tmp/nomad-device
cd /tmp/nomad-device
touch device01 && chmod 0777 device01
touch device02
```

It should be fingerprinted by the client agent after the `list_period`,
which you can check with:

```shell
nomad node status -json -self | jq '.NodeResources.Devices'
```

```json
[
  {
    "Attributes": null,
    "Instances": [
      {
        "HealthDescription": "Device has bad permissions \"-rwxrwxrwx\"",
        "Healthy": false,
        "ID": "device01",
        "Locality": null
      },
      {
        "HealthDescription": "",
        "Healthy": true,
        "ID": "device02",
        "Locality": null
      }
    ],
    "Name": "mock",
    "Type": "file",
    "Vendor": "nomad"
  }
]

```

The value to put in job specification
[device](https://developer.hashicorp.com/nomad/docs/job-specification/device)
block, or a quota specification,
is `"{Vendor}/{Type}/{Name}"` i.e. `"nomad/file/mock"`:

`job.nomad.hcl`:

```hcl
job "job" {
  group "grp" {
    task "tsk" {
      driver = "..."
      config {}
      resources {
        device "nomad/file/mock" {
          count = 1
        }
      }
    }
  }
}
```

`dev.quota.hcl`:

```hcl
name = "dev"
limit {
  region = "global"
  region_limit {
    device "nomad/file/mock" {
      count = 2 # to allow for deployments/reschedules
    }
  }
}
```
