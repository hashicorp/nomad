This package provides an example implementation of a device plugin forn
reference.

# Behavior

The example device plugin operates in three modes.

In `file` mode, The plugin will periodically scan the directory for changes and
will expose them via the streaming Fingerprint RPC. Device health is set to
unhealthy if the file has a specific filemode permission as described by the
config `unhealthy_perm`. Further statistics are also collected on the detected
devices.

In `static` mode, the plugin will accept a list of devices under the key
`device_config.` The plugin will use that configuration to model devices
and will expose them via the streaming Fingerprint RPC. The number of modeled
devices is the only statistic value reported are not streamed in static mode.

In `dynamic` mode, the plugin uses `attribute_config` and `device_config` to
generate fingerprints the same way as it does in `static` mode. However, the
plugin dynamically creates files during device reservation and attempts to
delete them after 2 minutes.

Device attributes can be configured via `attribute_config` configuration object
and will be applied to all devices.

# Installation

```shell
nomad_plugin_dir='/opt/nomad/plugins' # for example
go build -o $nomad_plugin_dir/nomad-device-example ./cmd
```

# Config

Example client agent config with our
[plugin](https://developer.hashicorp.com/nomad/docs/configuration/plugin) block
configured to file mode:

```hcl
client {
  enabled = true
}

plugin_dir = "/opt/nomad/plugins"

plugin "nomad-device-example" {
  config {
    plugin_mode = "file"
    dir            = "/tmp/nomad-device"
    list_period    = "1s"
    unhealthy_perm = "-rwxrwxrwx"
  }
}
```

Example client agent config with our [plugin](https://developer.hashicorp.com/nomad/docs/configuration/plugin) block
configured for static mode. Using the example below in dynamic mode would only
require changing the value of`plugin_mode` to `dynamic`:

```hcl
client {
  enabled = true
}

plugin_dir = "/opt/nomad/plugins"

plugin "nomad-device-example" {
  config {
    plugin_mode = "static"
    list_period    = "1s"
    attribute_config = [
      {
        attribute_name = "type"
        attribute_type = "string"
        attribute_value  = "file"
      },
      {
        attribute_name = "size"
        attribute_type = "int"
        attribute_value  = "30"
        unit           = "KB"
      },
    ]
    
    device_config = [
      {
        id = "w83u56j7d82983"
      },
      {
        id = "c93UBJ04dj2983"
        unhealthy= true
      },
    ]
  }
}
```
The valid configuration options are:

* `dir` (`string`: `"."`): The directory to scan for files that will represent
  fake devices. When running in `dynamic` mode, this value cannot include relative
  paths and may not contain any of the following directories:
  `bin, boot, dev, etc, lib, sbin, srv, proc, or Windows`

* `plugin_mode`(`string`: `"file"/"static"/"dynamic"`):
* `list_period` (`string`: `"5s"`): The interval to scan the directory for changes.
* `unhealthy_perm` (`string`: `"-rwxrwxrwx"`): The file mode permission that if set
  on a detected file will cause the device to be considered unhealthy.
* `attribute_config` (`block list`): optional static attribute configuration block.
  Configured attributes are applied to all devices.

  * `attribute_name` (`string`): required
  * `attribute_type` (`string`): required
  * `attribute_value` (`string`): required
  * `attribute_unit` (`string`): optional

* `device_config` (`block list`): optional static device configuration.
Cannot be set alongside `dir`.

  * `id` (`string`): required
  * `unhealthy` (`bool`): optional

# Usage
Running in `file` mode will require that you create files in the target directory.
You can skip this step if you are running in static or `dynamic` mode.
Create two instances of the device, one unhealthy (skip this step if running
in static or dynamic mode):

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
