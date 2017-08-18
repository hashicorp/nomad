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
  `script`<sup><small>1</small></sup>, `http` and `tcp` checks.

- `name` `(string: "<job>-<group>-<task>")` - Specifies the name of this
  service. If not supplied, this will default to the name of the job, group, and
  task concatenated together with a dash, like `"docs-example-server"`. Each
  service must have a unique name within the cluster. Names must adhere to
  [RFC-1123 §2.1](https://tools.ietf.org/html/rfc1123#section-2) and are limited
  to alphanumeric and hyphen characters (i.e. `[a-z0-9\-]`), and be less than 64
  characters in length.

    In addition to the standard [Nomad interpolation][interpolation], the
    following keys are also available:

    - `${JOB}` - the name of the job
    - `${GROUP}` - the name of the group
    - `${TASK}` - the name of the task
    - `${BASE}` - shorthand for `${JOB}-${GROUP}-${TASK}`

- `port` `(string: <required>)` - Specifies the label of the port on which this
  service is running. Note this is the _label_ of the port and not the port
  number. The port label must match one defined in the [`network`][network]
  stanza.

- `tags` `(array<string>: [])` - Specifies the list of tags to associate with
  this service. If this is not supplied, no tags will be assigned to the service
  when it is registered.

- `address_mode` `(string: "auto")` - Specifies what address (host or
  driver-specific) this service should advertise. `host` indicates the host IP
  and port. `driver` advertises the IP used in the driver (e.g. Docker's internal
  IP) and uses the ports specified in the port map. The default is `auto` which
  behaves the same as `host` unless the driver determines its IP should be used.
  This setting was added in Nomad 0.6 and only supported by the Docker driver.
  It will advertise the container IP if a network plugin is used (e.g. weave).

### `check` Parameters

Note that health checks run inside the task. If your task is a Docker container,
the script will run inside the Docker container. If your task is running in a
chroot, it will run in the chroot. Please keep this in mind when authoring check
scripts.

- `args` `(array<string>: [])` - Specifies additional arguments to the
  `command`. This only applies to script-based health checks.

- `command` `(string: <varies>)` - Specifies the command to run for performing
  the health check. The script must exit: 0 for passing, 1 for warning, or any
  other value for a failing health check. This is required for script-based
  health checks.

    ~> **Caveat:** The command must be the path to the command on disk, and no
    shell exists by default. That means operators like `||` or `&&` are not
    available. Additionally, all arguments must be supplied via the `args`
    parameter. To achieve the behavior of shell operators, specify the command
    as a shell, like `/bin/bash` and then use `args` to run the check.

- `initial_status` `(string: <enum>)` - Specifies the originating status of the
  service. Valid options are the empty string, `passing`, `warning`, and
  `critical`.

- `interval` `(string: <required>)` - Specifies the frequency of the health checks
  that Consul will perform. This is specified using a label suffix like "30s"
  or "1h". This must be greater than or equal to "1s"

- `method` `(string: "GET")` - Specifies the HTTP method to use for HTTP
  checks.

- `name` `(string: "service: <name> check")` - Specifies the name of the health
  check.

- `path` `(string: <varies>)` - Specifies the path of the HTTP endpoint which
  Consul will query to query the health of a service. Nomad will automatically
  add the IP of the service and the port, so this is just the relative URL to
  the health check endpoint. This is required for http-based health checks.

- `port` `(string: <required>)` - Specifies the label of the port on which the
  check will be performed. Note this is the _label_ of the port and not the port
  number. The port label must match one defined in the [`network`][network]
  stanza. If a port value was declared on the `service`, this will inherit from
  that value if not supplied. If supplied, this value takes precedence over the
  `service.port` value. This is useful for services which operate on multiple
  ports. Checks will *always use the host IP and ports*.

- `protocol` `(string: "http")` - Specifies the protocol for the http-based
  health checks. Valid options are `http` and `https`.

- `timeout` `(string: <required>)` - Specifies how long Consul will wait for a
  health check query to succeed. This is specified using a label suffix like
  "30s" or "1h". This must be greater than or equal to "1s"

- `type` `(string: <required>)` - This indicates the check types supported by
  Nomad. Valid options are `script`, `http`, and `tcp`.

- `tls_skip_verify` `(bool: false)` - Skip verifying TLS certificates for HTTPS
  checks. Requires Consul >= 0.7.2.

#### `header` Stanza

HTTP checks may include a `header` stanza to set HTTP headers. The `header`
stanza parameters have lists of strings as values. Multiple values will cause
the header to be set multiple times, once for each value.

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
    type     = "http"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
  }

  check {
    type     = "http"
    protocol = "https"
    port     = "lb"
    path     = "/_healthz"
    interval = "5s"
    timeout  = "2s"
    method   = "POST"
  }

  check {
    type     = "script"
    command  = "/usr/local/bin/pg-tools"
    args     = ["verify", "database" "prod", "up"]
    interval = "5s"
    timeout  = "2s"
  }
}
```

- - -

<sup><small>1</small></sup><small> Script checks are not supported for the
[qemu driver][qemu] since the Nomad client does not have access to the file
system of a task for that driver.</small>

[service-discovery]: /docs/service-discovery/index.html "Nomad Service Discovery"
[interpolation]: /docs/runtime/interpolation.html "Nomad Runtime Interpolation"
[network]: /docs/job-specification/network.html "Nomad network Job Specification"
[qemu]: /docs/drivers/qemu.html "Nomad qemu Driver"
