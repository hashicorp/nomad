---
layout: "docs"
page_title: "network Stanza - Job Specification"
sidebar_current: "docs-job-specification-network"
description: |-
  The "network" stanza specifies the networking requirements for the task,
  including the minimum bandwidth and port allocations.
---

# `network` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> group -> task -> resources -> **network**</code>
    </td>
  </tr>
</table>

The `network` stanza specifies the networking requirements for the task,
including the minimum bandwidth and port allocations. When scheduling jobs in
Nomad they are provisioned across your fleet of machines along with other jobs
and services. Because you don't know in advance what host your job will be
provisioned on, Nomad will provide your tasks with network configuration when
they start up.

Note that this document only applies to services that want to _listen_ on a
port. Batch jobs or services that only make outbound connections do not need to
allocate ports, since they will use any available interface to make an outbound
connection.


```hcl
job "docs" {
  group "example" {
    task "server" {
      resources {
        network {
          mbits = 200
          port "http" {}
          port "https" {}
          port "lb" {
            static = "8889"
          }
        }
      }
    }
  }
}
```

## `network` Parameters

- `mbits` `(int: 10)` - Specifies the bandwidth required in MBits.

- `port` <code>([Port](#port-parameters): nil)</code> - Specifies a TCP/UDP port
  allocation and can be used to specify both dynamic ports and reserved ports.

### `port` Parameters

- `static` `(int: nil)` - Specifies the static TCP/UDP port to allocate. If omitted, a dynamic port is chosen. We **do not recommend**  using static ports, except
  for `system` or specialized jobs like load balancers.

The label assigned to the port is used to identify the port in service
discovery, and used in the name of the environment variable that indicates
which port your application should bind to. For example:

```hcl
port "foo" {}
```

When the task starts, it will be passed the following environment variables:

- <tt>NOMAD_IP_foo</tt> - The IP to bind on for the given port label.
- <tt>NOMAD_PORT_foo</tt> - The port value for the given port label.
- <tt>NOMAD_ADDR_foo</tt> - A combined <tt>ip:port</tt> that can be used for convenience.

The label of the port is just text - it has no special meaning to Nomad.

## `network` Examples

The following examples only show the `network` stanzas. Remember that the
`network` stanza is only valid in the placements listed above.

### Bandwidth

This example specifies a resource requirement of 1 Gbit in bandwidth:

```hcl
network {
  mbits = 1000
}
```

### Dynamic Ports

This example specifies a dynamic port allocation for the port labeled "http".
Dynamic ports are allocated in a range from `20000` to `60000`.

Most services run in your cluster should use dynamic ports. This means that the
port will be allocated dynamically by the scheduler, and your service will have
to read an environment variable to know which port to bind to at startup.

```hcl
task "example" {
  resources {
    network {
      port "http" {}
      port "https" {}
    }
  }
}
```

```hcl
network {
  port "http" {}
}
```

### Static Ports

This example specifies a static port allocation for the port labeled "lb". Static
ports bind your job to a specific port on the host they' are placed on. Since
multiple services cannot share a port, the port must be open in order to place
your task.

```hcl
network {
  port "lb" {
    static = 6539
  }
}
```

### Mapped Ports

Some drivers (such as [Docker][docker-driver] and [QEMU][qemu-driver]) allow you
to map ports. A mapped port means that your application can listen on a fixed
port (it does not need to read the environment variable) and the dynamic port
will be mapped to the port in your container or virtual machine.

```hcl
task "example" {
  driver = "docker"

  config {
    port_map = {
      http = 8080
    }
  }

  resources {
    network {
      port "http" {}
    }
  }
}
```

The above example is for the Docker driver. The service is listening on port
`8080` inside the container. The driver will automatically map the dynamic port
to this service.

When the task is started, it is passed an additional environment variable named
`NOMAD_HOST_PORT_http` which indicates the host port that the HTTP service is
bound to.


[docker-driver]: /docs/drivers/docker.html "Nomad Docker Driver"
[qemu-driver]: /docs/drivers/qemu.html "Nomad QEMU Driver"
