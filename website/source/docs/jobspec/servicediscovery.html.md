---
layout: "docs"
page_title: "Service Discovery"
sidebar_current: "docs-jobspec-service-discovery"
description: |-
  Learn how to add service discovery to jobs
---

# Service Discovery

Nomad schedules workloads of various types across a cluster of generic hosts.
Because of this, placement is not known in advance and you will need to use
service discovery to connect tasks to other services deployed across your
cluster. Nomad integrates with [Consul](https://www.consul.io) to provide
service discovery and monitoring.

Note that in order to use Consul with Nomad, you will need to configure and
install Consul on your nodes alongside Nomad, or schedule it as a system job.
Nomad does not currently run Consul for you.

## Configuration

To configure Consul integration please see the Agent's configuration
[here](/docs/agent/config.html#consul_options).

## Service Definition Syntax

The service block in a task definition defines a service which Nomad will
register with Consul. Multiple service blocks are allowed in a task definition,
which allow registering multiple services for a task that exposes multiple
ports.

### Example

A brief example of a service definition in a task

```hcl
group "database" {
  task "mysql" {
    driver = "docker"

    service {
      tags = ["master", "mysql"]

      port = "db"

      check {
        type     = "tcp"
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

    resources {
      cpu    = 500
      memory = 1024

      network {
        mbits = 10
        port "db" {}
      }
    }
  }
}
```

* `Name`: An explicit name for the Service. Nomad will replace `${JOB}`,
  `${TASKGROUP}` and `${TASK}` by the name of the job, task group or task,
  respectively. `${BASE}` expands to the equivalent of
  `${JOB}-${TASKGROUP}-${TASK}`, and is the default name for a Service.
  Each service defined for a given task must have a distinct name, so if
  a task has multiple services only one of them can use the default name
  and the others must be explicitly named. Names must adhere to
  [RFC-1123 §2.1](https://tools.ietf.org/html/rfc1123#section-2) and are
  limited to alphanumeric and hyphen characters (i.e. `[a-z0-9\-]`), and be
  less than 64 characters in length.

* `tags`: A list of tags associated with this Service. String interpolation is
  supported in tags.

* `port`: `port` is optional and is used to associate a port with the service.
  If specified, the port label must match one defined in the resources block.
  This could be a label of either a dynamic or a static port.

* `check`: A check block defines a health check associated with the service.
  Multiple check blocks are allowed for a service. Nomad supports the `script`,
  `http` and `tcp` Consul Checks. Script checks are not supported for the qemu
  driver since the Nomad client doesn't have access to the file system of a
  task using the Qemu driver.

### Check Syntax

* `type`: This indicates the check types supported by Nomad. Valid options are
  currently `script`, `http` and `tcp`.

* `name`: The name of the health check.

* `interval`: This indicates the frequency of the health checks that Consul will
  perform.

* `timeout`: This indicates how long Consul will wait for a health check query
  to succeed.

* `path`: The path of the http endpoint which Consul will query to query the
  health of a service if the type of the check is `http`. Nomad will add the IP
  of the service and the port, users are only required to add the relative URL
  of the health check endpoint.

* `protocol`: This indicates the protocol for the http checks. Valid options
  are `http` and `https`. We default it to `http`.

* `command`: This is the command that the Nomad client runs for doing script based
  health check.

* `args`: Additional arguments to the `command` for script based health checks.

## Assumptions

* Consul 0.6.4 or later is needed for using the Script checks.

* Consul 0.6.0 or later is needed for using the TCP checks.

* The service discovery feature in Nomad depends on operators making sure that
  the Nomad client can reach the Consul agent.

* Nomad assumes that it controls the life cycle of all the externally
  discoverable services running on a host.

* Tasks running inside Nomad also need to reach out to the Consul agent if
  they want to use any of the Consul APIs. Ex: A task running inside a docker
  container in the bridge mode won't be able to talk to a Consul Agent running
  on the loopback interface of the host since the container in the bridge mode
  has it's own network interface and doesn't see interfaces on the global
  network namespace of the host. There are a couple of ways to solve this, one
  way is to run the container in the host networking mode, or make the Consul
  agent listen on an interface in the network namespace of the container.
