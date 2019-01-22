---
layout: "docs"
page_title: "Drivers: LXC"
sidebar_current: "docs-external-plugins-lxc"
description: |-
  The LXC task driver is used to run application containers using LXC.
---

# LXC Driver

Name: `lxc`

The `lxc` driver provides an interface for using LXC for running application
containers.

~> Nomad 0.9 does not maintain backward compatibility for the external LXC driver plugin when it comes to client configuration syntax. With Nomad 0.9, you must use the new [plugin syntax][plugin]. See [plugin options][plugin-options] below for an example.

## Task Configuration

```hcl
task "busybox" {
  driver = "lxc"

  config {
    log_level = "trace"
    verbosity = "verbose"
    template = "/usr/share/lxc/templates/lxc-busybox"
  }
}
```

The `lxc` driver supports the following configuration in the job spec:

* `template` - The LXC template to run.

    ```hcl
    config {
      template = "/usr/share/lxc/templates/lxc-alpine"
    }
    ```

* `log_level` - (Optional) LXC library's logging level. Defaults to `error`.
  Must be one of `trace`, `debug`, `info`, `warn`, or `error`.

    ```hcl
    config {
      log_level = "debug"
    }
    ```

* `verbosity` - (Optional) Enables extra verbosity in the LXC library's
  logging. Defaults to `quiet`. Must be one of `quiet` or `verbose`.

    ```hcl
    config {
      verbosity = "quiet"
    }
    ```

* `volumes` - (Optional) A list of `host_path:container_path` strings to bind-mount
  host paths to container paths. Mounting host paths outside of the allocation
  directory can be disabled on clients by setting the `lxc.volumes.enabled`
  option set to false. This will limit volumes to directories that exist inside
  the allocation directory.

  Note that unlike the similar option for the docker driver, this
  option must not have an absolute path as the `container_path`
  component. This will cause an error when submitting a job.

  Setting this does not affect the standard bind-mounts of `alloc`,
  `local`, and `secrets`, which are always created.

```hcl
config {
  volumes = [
    # Use absolute paths to mount arbitrary paths on the host
    "/path/on/host:path/in/container",

    # Use relative paths to rebind paths already in the allocation dir
    "relative/to/task:also/in/container"
  ]
}
```

## Networking

Currently the `lxc` driver only supports host networking. See the `none`
networking type in the `lxc.container.conf` [manual][lxc_man] for more
information.

## Client Requirements

The `lxc` driver requires the following:

* 64-bit Linux host
* The `linux_amd64_lxc` Nomad binary
* `liblxc` to be installed
* `lxc-templates` to be installed

## Plugin Options<a id="plugin_options"></a>

* `enabled` - The `lxc` driver may be disabled on hosts by setting this option to `false` (defaults to `true`).

* `volumes_enabled` - Specifies whether host can bind-mount host paths to container paths (defaults to `false`). 

* `lxc_path` - The location in which all containers are stored (defaults to
  `/var/lib/lxc`). See [`lxc-create`][lxc-create] for more details. 

An example of using these plugin options with the new [plugin
syntax][plugin] is shown below:

```hcl
plugin "nomad-driver-lxc" {
  config {
    enabled = true
    volumes_enabled = true
    lxc_path = "/var/lib/lxc"
  }
}
```
Please note the plugin name should match whatever name you have specified for
the external driver in the [`data_dir`][data_dir]`/plugins` directory.

## Client Configuration

~> Only use this section for pre-0.9 releases of Nomad. If you are using Nomad
0.9 or above, please see [plugin options][plugin-options]

The `lxc` driver has the following [client configuration
options](/docs/configuration/client.html#options):

* `lxc.enable` - The `lxc` driver may be disabled on hosts by setting this
  option to `false` (defaults to `true`).

## Client Attributes

The `lxc` driver will set the following client attributes:

* `driver.lxc` - Set to `1` if LXC is found  and enabled on the host node.
* `driver.lxc.version` - Version of `lxc` e.g.: `1.1.0`.

## Resource Isolation

This driver supports CPU and memory isolation via the `lxc` library. Network
isolation is not supported as of now.

[data_dir]: /docs/configuration/index.html#data_dir
[lxc-create]: https://linuxcontainers.org/lxc/manpages/man1/lxc-create.1.html
[lxc_man]: https://linuxcontainers.org/lxc/manpages/man5/lxc.container.conf.5.html#lbAM
[plugin]: /docs/configuration/plugin.html
[plugin-options]: #plugin_options
