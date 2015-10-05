---
layout: "docs"
page_title: "Discovery with Consul"
sidebar_current: "docs-discovery-consul"
description: >
  Nomad integrates with Consul to provide seamless service discovery for
  running jobs and tasks.
---

# Consul Discovery Integration

[Consul](https://consul.io) is a tool for service discovery produced by
HashiCorp. It is distributed, highly available, and fault tolerant. Consul
provides a very convenient DNS interface which can be used to query for
registered nodes and services. Nomad can leverage Consul's service discovery
abilities using the built-in Consul discovery provider.

In addition to service discovery, Consul also provides rich health checking
abilities which directly tie into discovery results. A gossip-based failure
detector is used to provide near real-time updates of node status and
failures, and route away from them if possible. This means that by simply
registering a Nomad task with Consul, you are automatically able to query and
load balance requests using the DNS interface, and avoid routing to unhealthy
instances.

## Configuration

This discovery provider allows specifying connectivity settings using Nomad's
[client options](/docs/agent/config.html#options). The following settins are
configurable:

* `discovery.consul.enable`: If set to `true`, enables the Consul discovery
  back-end. Defaults to false.
* `discovery.consul.address`: Specifies the address of the Consul agent to
  register with. Defaults to `127.0.0.1:8500`.
* `discovery.consul.scheme`: Specifies the address scheme to use while
  connecting. Defaults to `http`.
* `discovery.consul.token`: Specifies a specific Consul ACL token to use for
  performing service registration operations.

## Registration with a Consul agent

This discovery back-end registers services against a local Consul agent. What
this means is that the Consul agent's information about the local node,
including its IP address, are used to register the service. This enables
leveraging the scalability and health checking abilities that Consul has to
offer. For more information on how Consul is architected, see the
[consul architecture](https://consul.io/docs/internals/architecture.html) page.

## Service names with Consul and Nomad

The Consul discovery backend registers tasks using a common task prefix. Dynamic
ports are registered as separate services by joining the task with the port
label. For example, a job `job1` with task group `group1`, and a task named
`task1` would, by default, register as `job1-group1-task1`. If that same task
used a dynamic port, labeled `https`, it would also be registered as
`job1-group1-task1-https`.

The default prefix is sufficient for uniquely identifying specific tasks,
however it may be undesirable if a shorter, unique service name can be provided.
These shorter names may be specified using the
[discovery](/docs/jobspec/index.html#discovery) argument within a given task
definition. For example, if the `discovery = "web"` option is given in the task
definition, then the entries would instead be registered as `web` and
`web-https`. **Important**: Take care when specifying an explicit discovery name.
If the name is not unique, multiple, unrelated tasks may be discovered by the
same name.
