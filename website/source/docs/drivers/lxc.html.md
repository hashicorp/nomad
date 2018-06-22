---
layout: "docs"
page_title: "Drivers: LXC"
sidebar_current: "docs-drivers-lxc"
description: |-
  The LXC task driver is used to run application containers using LXC.
---

# LXC Driver

Name: `lxc`

The `lxc` driver provides an interface for using LXC for running application
containers.

!> **Experimental!** Currently, the LXC driver supports launching containers
via templates but only supports host networking. If both an LXC image and the
host it is run on use upstart or systemd, shutdown signals may be passed from
the container to the host.

~> LXC is only enabled in the special `linux_amd64_lxc` build of Nomad because
it links to the `liblxc` system library. Use the `lxc` build tag if compiling
Nomad yourself.

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
networking type in the [`lxc.container.conf` manual][lxc_man] for more
information.

[lxc_man]: https://linuxcontainers.org/lxc/manpages/man5/lxc.container.conf.5.html#lbAM

## Client Requirements

The `lxc` driver requires the following:

* 64-bit Linux host
* The `linux_amd64_lxc` Nomad binary
* `liblxc` to be installed
* `lxc-templates` to be installed

## Client Configuration

* `lxc.enable` - The `lxc` driver may be disabled on hosts by setting this
  [client configuration][/docs/configuration/client.html##options-parameters]
  option to `false` (defaults to `true`).

## Client Attributes

The `lxc` driver will set the following client attributes:

* `driver.lxc` - Set to `1` if LXC is found  and enabled on the host node.
* `driver.lxc.version` - Version of `lxc` e.g.: `1.1.0`.

## Resource Isolation

This driver supports CPU and memory isolation via the `lxc` library. Network
isolation is not supported as of now.
