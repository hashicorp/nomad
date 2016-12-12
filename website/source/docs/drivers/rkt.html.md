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
  variables](/docs/runtime/interpolation.html) will be interpreted before
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

* `dns_servers` - (Optional) A list of DNS servers to be used in the container.
  Alternatively a list containing just `host` or `none`. `host` uses the host's
  `resolv.conf` while `none` forces use of the image's name resolution configuration.

* `dns_search_domains` - (Optional) A list of DNS search domains to be used in
   the containers.

* `net` - (Optional) A list of networks to be used by the containers

* `port_map` - (Optional) A key/value map of port to be used by the container.
   port name in the image manifest file needs to be specified for the value. For example:

   ```
    port_map {
            app = "8080-tcp"
    }
   ```

   See below for more details.

* `debug` - (Optional) Enable rkt command debug option.

* `volumes` - (Optional) A list of `host_path:container_path` strings to bind
  host paths to container paths.

    ```hcl
    config {
      volumes = ["/path/on/host:/path/in/container"]
    }
    ```

## Networking

The `rkt` can specify `--net` and `--port` for the rkt client. Hence, there are two ways to use host ports by
using `--net=host` or `--port=PORT` with your network.

Example:

```
task "redis" {
	# Use rkt to run the task.
	driver = "rkt"

	config {
		# Use docker image with port defined
		image = "docker://redis:latest"
		port_map {
			app = "6379-tcp"
		}
	}

	service {
		port = "app"
	}

	resources {
		network {
			mbits = 10
			port "app" {
			    static = 12345
			}
		}
	}
}
```

### Allocating Ports

You can allocate ports to your task using the port syntax described on the
[networking page](/docs/job-specification/network.html).

When you use port allocation, the image manifest needs to declare public ports and host has configured network.
For more information, please refer to [rkt Networking](https://coreos.com/rkt/docs/latest/networking/overview.html).

## Client Requirements

The `rkt` driver requires rkt to be installed and in your system's `$PATH`.
The `trust_prefix` must be accessible by the node running Nomad. This can be an
internal source, private to your cluster, but it must be reachable by the client
over HTTP.

## Client Configuration

The `rkt` driver has the following [client configuration
options](/docs/agent/configuration/client.html#options):

* `rkt.volumes.enabled`: Defaults to `true`. Allows tasks to bind host paths
  (`volumes`) inside their container. Binding relative paths is always allowed
  and will be resolved relative to the allocation's directory.


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
