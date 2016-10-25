---
layout: "docs"
page_title: "Drivers: Rkt"
sidebar_current: "docs-drivers-rkt"
description: |-
  The rkt task driver is used to run application containers using rkt.
---

# Rkt Driver

Name: `rkt`

The `rkt` driver provides an interface for using CoreOS rkt for running
application containers.

~> **Experimental!** Currently, the rkt driver supports launching containers but
does not support dynamic ports. This can lead to port conflicts and as such,
this driver is being marked as experimental and should be used with care.

## Task Configuration

```hcl
task "webservice" {
  driver = "rkt"

  config {
    image = "redis:3.2"
  }
}    
```

The `rkt` driver supports the following configuration in the job spec:

* `image` - The image to run. May be specified by name, hash, ACI address
  or docker registry.

    ```hcl
    config {
      image = "https://hub.docker.internal/redis:3.2"
    }
    ```

* `command` - (Optional) A command to execute on the ACI.

    ```hcl
    config {
      command = "my-command"
    }
    ```

* `args` - (Optional) A list of arguments to the optional `command`. References
  to environment variables or any [interpretable Nomad
  variables](/docs/jobspec/interpreted.html) will be interpreted before
  launching the task.

    ```hcl
    config {
      args = [
        "-bind", "${NOMAD_PORT_http}",
        "${nomad.datacenter}",
        "${MY_ENV}",
        "${meta.foo}",
      ]
    }
    ```

* `trust_prefix` - (Optional) The trust prefix to be passed to rkt. Must be
  reachable from the box running the nomad agent. If not specified, the image is
  run without verifying the image signature.

* `dns_servers` - (Optional) A list of DNS servers to be used in the containers.

* `dns_search_domains` - (Optional) A list of DNS search domains to be used in
   the containers.

* `debug` - (Optional) Enable rkt command debug option.

* `volumes` - (Optional) A list of `host_path:container_path` strings to bind
  host paths to container paths. Can only be run on clients with the
  `rkt.volumes.enabled` option set to true.

    ```hcl
    config {
      volumes = ["/path/on/host:/path/in/container"]
    }
    ```

## Client Requirements

The `rkt` driver requires rkt to be installed and in your system's `$PATH`.
The `trust_prefix` must be accessible by the node running Nomad. This can be an
internal source, private to your cluster, but it must be reachable by the client
over HTTP.

## Client Configuration

The `rkt` driver has the following [client configuration
options](/docs/agent/config.html#options):

* `rkt.volumes.enabled`: Defaults to `false`. Allows tasks to bind host paths
  (`volumes`) inside their container.  Disabled by default as it removes the
  isolation between containers' data.

## Client Attributes

The `rkt` driver will set the following client attributes:

* `driver.rkt` - Set to `1` if rkt is found on the host node. Nomad determines
  this by executing `rkt version` on the host and parsing the output
* `driver.rkt.version` - Version of `rkt` eg: `1.1.0`. Note that the minimum required
  version is `1.0.0`
* `driver.rkt.appc.version` - Version of `appc` that `rkt` is using eg: `1.1.0`

Here is an example of using these properties in a job file:

```hcl
job "docs" {
  # Only run this job where the rkt version is higher than 0.8.
  constraint {
    attribute = "${driver.rkt.version}"
    operator  = ">"
    value     = "1.2"
  }
}
```

## Resource Isolation

This driver supports CPU and memory isolation by delegating to `rkt`. Network
isolation is not supported as of now.
