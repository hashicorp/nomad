---
layout: "docs"
page_title: "service Stanza - Job Specification"
sidebar_current: "docs-job-specification-service"
description: |-
  The "service" stanza instructs Nomad to register the task as a service using
  the service discovery integration.
---

# `service` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> **service**</code>
    </td>
  </tr>
</table>

The `service` stanza instructs Nomad to register the task as a service using the
service discovery integration. This section of the documentation will discuss the
configuration, but please also read the
[Nomad service discovery documentation][service-discovery] for more detailed
information about the integration.

```hcl
job "docs" {
  group "example" {
    task "server" {
      service {
        tags = ["leader", "mysql"]

        port = "db"

        check {
          type     = "tcp"
          port     = "db"
          interval = "10s"
          timeout  = "2s"
        }

        check {
          type     = "script"
          name     = "check_table"
          command  = "/usr/local/bin/check_mysql_table_status"
          args     = ["--verbose"]
          interval = "60s"
          timeout  = "5s"

          check_restart {
            limit = 3
            grace = "90s"
            ignore_warnings = false
          }
        }
      }
    }
  }
}
```

This section of the documentation only covers the job file options for
configuring service discovery. For more information on the setup and
configuration to integrate Nomad with service discovery, please see the
[Nomad service discovery documentation][service-discovery]. There are steps you
must take to configure Nomad. Simply adding this configuration to your job file
does not automatically enable service discovery.

## `service` Parameters

- `check` <code>([Check](#check-parameters): nil)</code> - Specifies a health
  check associated with the service. This can be specified multiple times to
  define multiple checks for the service. At this time, Nomad supports the
  `grpc`, `http`, `script`<sup><small>1</small></sup>, and `tcp` checks.

- `name` `(string: "<job>-<group>-<task>")` - Specifies the name this service
  will be advertised as in Consul.  If not supplied, this will default to the
  name of the job, group, and task concatenated together with a dash, like
  `"docs-example-server"`. Each service must have a unique name within the
  cluster. Names must adhere to [RFC-1123
  §2.1](https://tools.ietf.org/html/rfc1123#section-2) and are limited to
  alphanumeric and hyphen characters (i.e. `[a-z0-9\-]`), and be less than 64
  characters in length.

    In addition to the standard [Nomad interpolation][interpolation], the
    following keys are also available:

    - `${JOB}` - the name of the job
    - `${GROUP}` - the name of the group
    - `${TASK}` - the name of the task
    - `${BASE}` - shorthand for `${JOB}-${GROUP}-${TASK}`
    
    Validation of the name occurs in two parts. When the job is registered, an initial validation pass checks that
    the service name adheres to RFC-1123 §2.1 and the length limit, excluding any variables requiring interpolation. 
    Once the client receives the service and all interpretable values are available, the service name will be 
    interpolated and revalidated. This can cause certain service names to pass validation at submit time but fail 
    at runtime.
    
- `port` `(string: <optional>)` - Specifies the port to advertise for this
  service. The value of `port` depends on which [`address_mode`](#address_mode)
  is being used:

  - `driver` - Advertise the port determined by the driver (eg Docker or rkt).
    The `port` may be a numeric port or a port label specified in the driver's
    `port_map`.

  - `host` - Advertise the host port for this service. `port` must match a port
    _label_ specified in the [`network`][network] stanza.

- `tags` `(array<string>: [])` - Specifies the list of tags to associate with
  this service. If this is not supplied, no tags will be assigned to the service
  when it is registered.

- `canary_tags` `(array<string>: [])` - Specifies the list of tags to associate with
  this service when the service is part of an allocation that is currently a
  canary. Once the canary is promoted, the registered tags will be updated to
  those specified in the `tags` parameter. If this is not supplied, the
  registered tags will be equal to that of the `tags parameter.

- `address_mode` `(string: "auto")` - Specifies what address (host or
  driver-specific) this service should advertise.  This setting is supported in
  Docker since Nomad 0.6 and rkt since Nomad 0.7. See [below for
  examples.](#using-driver-address-mode) Valid options are:

  - `auto` - Allows the driver to determine whether the host or driver address
    should be used. Defaults to `host` and only implemented by Docker. If you
    use a Docker network plugin such as weave, Docker will automatically use
    its address.

  - `driver` - Use the IP specified by the driver, and the port specified in a
    port map. A numeric port may be specified since port maps aren't required
    by all network plugins. Useful for advertising SDN and overlay network
    addresses. Task will fail if driver network cannot be determined. Only
    implemented for Docker and rkt.

  - `host` - Use the host IP and port.

### `check` Parameters

Note that health checks run inside the task. If your task is a Docker container,
the script will run inside the Docker container. If your task is running in a
chroot, it will run in the chroot. Please keep this in mind when authoring check
scripts.

- `address_mode` `(string: "host")` - Same as `address_mode` on `service`.
  Unlike services, checks do not have an `auto` address mode as there's no way
  for Nomad to know which is the best address to use for checks. Consul needs
  access to the address for any HTTP or TCP checks. Added in Nomad 0.7.1. See
  [below for details.](#using-driver-address-mode) Unlike `port`, this setting
  is *not* inherited from the `service`.

- `args` `(array<string>: [])` - Specifies additional arguments to the
  `command`. This only applies to script-based health checks.

- `check_restart` - See [`check_restart` stanza][check_restart_stanza].

- `command` `(string: <varies>)` - Specifies the command to run for performing
  the health check. The script must exit: 0 for passing, 1 for warning, or any
  other value for a failing health check. This is required for script-based
  health checks.

    ~> **Caveat:** The command must be the path to the command on disk, and no
    shell exists by default. That means operators like `||` or `&&` are not
    available. Additionally, all arguments must be supplied via the `args`
    parameter. To achieve the behavior of shell operators, specify the command
    as a shell, like `/bin/bash` and then use `args` to run the check.

- `grpc_service` `(string: <optional>)` - What service, if any, to specify in
  the gRPC health check. gRPC health checks require Consul 1.0.5 or later.

- `grpc_use_tls` `(bool: false)` - Use TLS to perform a gRPC health check. May
  be used with `tls_skip_verify` to use TLS but skip certificate verification.

- `initial_status` `(string: <enum>)` - Specifies the originating status of the
  service. Valid options are the empty string, `passing`, `warning`, and
  `critical`.

- `interval` `(string: <required>)` - Specifies the frequency of the health checks
  that Consul will perform. This is specified using a label suffix like "30s"
  or "1h". This must be greater than or equal to "1s"

- `method` `(string: "GET")` - Specifies the HTTP method to use for HTTP
  checks.

- `name` `(string: "service: <name> check")` - Specifies the name of the health
  check. If the name is not specified Nomad generates one based on the service name.
  If you have more than one check you must specify the name.

- `path` `(string: <varies>)` - Specifies the path of the HTTP endpoint which
  Consul will query to query the health of a service. Nomad will automatically
  add the IP of the service and the port, so this is just the relative URL to
  the health check endpoint. This is required for http-based health checks.

- `port` `(string: <varies>)` - Specifies the label of the port on which the
  check will be performed. Note this is the _label_ of the port and not the port
  number unless `address_mode = driver`. The port label must match one defined
  in the [`network`][network] stanza. If a port value was declared on the
  `service`, this will inherit from that value if not supplied. If supplied,
  this value takes precedence over the `service.port` value. This is useful for
  services which operate on multiple ports. `grpc`, `http`, and `tcp` checks
  require a port while `script` checks do not. Checks will use the host IP and
  ports by default. In Nomad 0.7.1 or later numeric ports may be used if
  `address_mode="driver"` is set on the check.

- `protocol` `(string: "http")` - Specifies the protocol for the http-based
  health checks. Valid options are `http` and `https`.

- `timeout` `(string: <required>)` - Specifies how long Consul will wait for a
  health check query to succeed. This is specified using a label suffix like
  "30s" or "1h". This must be greater than or equal to "1s"

- `type` `(string: <required>)` - This indicates the check types supported by
  Nomad. Valid options are `grpc`, `http`, `script`, and `tcp`. gRPC health
  checks require Consul 1.0.5 or later.

- `tls_skip_verify` `(bool: false)` - Skip verifying TLS certificates for HTTPS
  checks. Requires Consul >= 0.7.2.

#### `header` Stanza

HTTP checks may include a `header` stanza to set HTTP headers. The `header`
stanza parameters have lists of strings as values. Multiple values will cause
the header to be set multiple times, once for each value.

```hcl
service {
  # ...
  check {
    type     = "http"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
    header {
      Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
    }
  }
}
```


## `service` Examples

The following examples only show the `service` stanzas. Remember that the
`service` stanza is only valid in the placements listed above.

### Basic Service

This example registers a service named "load-balancer" with no health checks.

```hcl
service {
  name = "load-balancer"
  port = "lb"
}
```

This example must be accompanied by a [`network`][network] stanza which defines
a static or dynamic port labeled "lb". For example:

```hcl
resources {
  network {
    mbits = 10
    port "lb" {}
  }
}
```

### Check with Bash-isms

This example shows a common mistake and correct behavior for custom checks.
Suppose a health check like this:

```shell
$ test -f /tmp/file.txt
```

In this example `test` is not actually a command (binary) on the system; it is a
built-in shell function to bash. Thus, the following **would not work**:

```hcl
service {
  check {
    type    = "script"
    command = "test -f /tmp/file.txt" # THIS IS NOT CORRECT
  }
}
```

Nomad will attempt to find an executable named `test` on your system, but it
does not exist. It is actually just a function of bash. Additionally, it is not
possible to specify the arguments in a single string. Here is the correct
solution:

```hcl
service {
  check {
    type    = "script"
    command = "/bin/bash"
    args    = ["-c", "test -f /tmp/file.txt"]
  }
}
```

The `command` is actually `/bin/bash`, since that is the actual process we are
running. The arguments to that command are the script itself, which each
argument provided as a value to the `args` array.

### HTTP Health Check

This example shows a service with an HTTP health check. This will query the
service on the IP and port registered with Nomad at `/_healthz` every 5 seconds,
giving the service a maximum of 2 seconds to return a response, and include an
Authorization header. Any non-2xx code is considered a failure.

```hcl
service {
  check {
    type     = "http"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
    header {
      Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
    }
  }
}
```

### Multiple Health Checks

This example shows a service with multiple health checks defined. All health
checks must be passing in order for the service to register as healthy.

```hcl
service {
  check {
    name     = "HTTP Check"
    type     = "http"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
  }

  check {
    name     = "HTTPS Check"
    type     = "http"
    protocol = "https"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
    method   = "POST"
  }

  check {
    name     = "Postgres Check"
    type     = "script"
    command  = "/usr/local/bin/pg-tools"
    args     = ["verify", "database" "prod", "up"]
    interval = "5s"
    timeout  = "2s"
  }
}
```

### gRPC Health Check

gRPC health checks use the same host and port behavior as `http` and `tcp`
checks, but gRPC checks also have an optional gRPC service to health check. Not
all gRPC applications require a service to health check. gRPC health checks
require Consul 1.0.5 or later.

```hcl
service {
  check {
    type            = "grpc"
    port            = "rpc"
    interval        = "5s"
    timeout         = "2s"
    grpc_service    = "example.Service"
    grpc_use_tls    = true
    tls_skip_verify = true
  }
}
```

In this example Consul would health check the `example.Service` service on the
`rpc` port defined in the task's [network resources][network] stanza.  See
[Using Driver Address Mode](#using-driver-address-mode) for details on address
selection.

### Using Driver Address Mode

The [Docker](/docs/drivers/docker.html#network_mode) and
[rkt](/docs/drivers/rkt.html#net) drivers support the `driver` setting for the
`address_mode` parameter in both `service` and `check` stanzas. The driver
address mode allows advertising and health checking the IP and port assigned to
a task by the driver. This way if you're using a network plugin like Weave with
Docker, you can advertise the Weave address in Consul instead of the host's
address.

For example if you were running the example Redis job in an environment with
Weave but Consul was running on the host you could use the following
configuration:

```hcl
job "example" {
  datacenters = ["dc1"]
  group "cache" {

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
        network_mode = "weave"
        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
        network {
          mbits = 10
          port "db" {}
        }
      }

      service {
        name = "weave-redis"
        port = "db"
        check {
          name     = "host-redis-check"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

No explicit `address_mode` required!

Services default to the `auto` address mode. When a Docker network mode other
than "host" or "bridge" is used, services will automatically advertise the
driver's address (in this case Weave's). The service will advertise the
container's port: 6379.

However since Consul is often run on the host without access to the Weave
network, `check` stanzas default to `host` address mode. The TCP check will run
against the host's IP and the dynamic host port assigned by Nomad.

Note that the `check` still inherits the `service` stanza's `db` port label,
but each will resolve the port label according to their address mode.

If Consul has access to the Weave network the job could be configured like
this:

```hcl
job "example" {
  datacenters = ["dc1"]
  group "cache" {

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
        network_mode = "weave"
        # No port map required!
      }

      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
        network {
          mbits = 10
        }
      }

      service {
        name = "weave-redis"
        port = 6379
        address_mode = "driver"
        check {
          name     = "host-redis-check"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
          port     = 6379
          
          address_mode = "driver"
        }
      }
    }
  }
}
```

In this case Nomad doesn't need to assign Redis any host ports. The `service`
and `check` stanzas can both specify the port number to advertise and check
directly since Nomad isn't managing any port assignments.

### IPv6 Docker containers

The [Docker](/docs/drivers/docker.html#advertise_ipv6_address) driver supports the
`advertise_ipv6_address` parameter in it's configuration.

Services will automatically advertise the IPv6 address when `advertise_ipv6_address` 
is used.

Unlike services, checks do not have an `auto` address mode as there's no way
for Nomad to know which is the best address to use for checks. Consul needs
access to the address for any HTTP or TCP checks.

So you have to set `address_mode` parameter in the `check` stanza to `driver`. 

For example using `auto` address mode:

```hcl
job "example" {
  datacenters = ["dc1"]
  group "cache" {

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
        advertise_ipv6_address = true
        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
        network {
          mbits = 10
          port "db" {}
        }
      }

      service {
        name = "ipv6-redis"
        port = db
        check {
          name     = "ipv6-redis-check"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
          port     = db
          address_mode = "driver"
        }
      }
    }
  }
}
```

Or using `address_mode=driver` for `service` and `check` with numeric ports:

```hcl
job "example" {
  datacenters = ["dc1"]
  group "cache" {

    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"
        advertise_ipv6_address = true
        # No port map required!
      }

      resources {
        cpu    = 500 # 500 MHz
        memory = 256 # 256MB
        network {
          mbits = 10
        }
      }

      service {
        name = "ipv6-redis"
        port = 6379
        address_mode = "driver"
        check {
          name     = "ipv6-redis-check"
          type     = "tcp"
          interval = "10s"
          timeout  = "2s"
          port     = 6379
          address_mode = "driver"
        }
      }
    }
  }
}
```

The `service` and `check` stanzas can both specify the port number to 
advertise and check directly since Nomad isn't managing any port assignments.


- - -

<sup><small>1</small></sup><small> Script checks are not supported for the
[qemu driver][qemu] since the Nomad client does not have access to the file
system of a task for that driver.</small>

[check_restart_stanza]: /docs/job-specification/check_restart.html "check_restart stanza"
[consul_grpc]: https://www.consul.io/api/agent/check.html#grpc
[service-discovery]: /guides/operations/consul-integration/index.html#service-discovery/index.html "Nomad Service Discovery"
[interpolation]: /docs/runtime/interpolation.html "Nomad Runtime Interpolation"
[network]: /docs/job-specification/network.html "Nomad network Job Specification"
[qemu]: /docs/drivers/qemu.html "Nomad qemu Driver"
[restart_stanza]: /docs/job-specification/restart.html "restart stanza"
